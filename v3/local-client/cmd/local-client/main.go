package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"videowithyou/v3/internal/hostserver"
	"videowithyou/v3/internal/install"
	"videowithyou/v3/internal/localcert"
	"videowithyou/v3/internal/logging"
	"videowithyou/v3/internal/roomserver"
	"videowithyou/v3/internal/sshtunnel"
	"videowithyou/v3/local-client/internal/client"
	"videowithyou/v3/local-client/internal/config"
	"videowithyou/v3/local-client/internal/embedded"
	"videowithyou/v3/local-client/internal/extws"
)

const version = "3.0.2"

func main() {
	if err := run(); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "VideoWithYou 启动失败: %v\n", err)
	}
}

func run() error {
	defaultConfigPath, err := config.DefaultConfigPath()
	if err != nil {
		defaultConfigPath = filepath.Join(".", "config.json")
	}
	flags := flag.NewFlagSet("VideoWithYou", flag.ContinueOnError)
	configPath := flags.String("config", defaultConfigPath, "configuration file path")
	initOwner := flags.Bool("init-owner", false, "initialize this computer as the hybrid server")
	importProfile := flags.String("import-profile", "", "import a .vwyprofile file")
	exportProfile := flags.String("export-profile", "", "export a client .vwyprofile file")
	extractExtension := flags.Bool("extract-extension", false, "extract the bundled browser extension and exit")
	installApp := flags.Bool("install", false, "copy this executable into the current user's LocalAppData")
	enableAutostart := flags.Bool("autostart", false, "enable startup after --install")
	showVersion := flags.Bool("version", false, "print version and exit")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	if *showVersion {
		fmt.Printf("VideoWithYou v%s\n", version)
		return nil
	}

	logDir, logDirErr := config.DefaultLogDir()
	if logDirErr != nil {
		logDir = filepath.Join(".", "logs")
	}
	logger, closer, logErr := logging.New(logDir)
	if logErr != nil {
		logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)
		fmt.Fprintf(os.Stderr, "无法创建日志文件，将仅输出到控制台: %v\n", logErr)
	} else {
		defer closer.Close()
	}
	logger.Printf("[启动] VideoWithYou v%s config=%s", version, *configPath)

	cfg, err := loadConfigResilient(*configPath, logger)
	if err != nil {
		return err
	}
	runtimeDir, err := config.RuntimeDir()
	if err != nil {
		return err
	}
	ownerProfilePath := filepath.Join(runtimeDir, "VideoWithYou-client.vwyprofile")

	if strings.TrimSpace(*importProfile) != "" {
		profile, err := config.LoadProfile(*importProfile)
		if err != nil {
			return fmt.Errorf("读取客户端配置失败: %w", err)
		}
		if err := cfg.ApplyProfile(profile); err != nil {
			return fmt.Errorf("导入客户端配置失败: %w", err)
		}
		if err := config.SaveConfig(*configPath, cfg); err != nil {
			return err
		}
		logger.Printf("[配置] 客户端配置已导入 path=%s", *importProfile)
	}

	if *initOwner {
		if err := cfg.EnableOwnerMode(runtimeDir); err != nil {
			return err
		}
	}
	if cfg.Server.Enabled {
		wasACME := strings.EqualFold(strings.TrimSpace(cfg.Server.TLS.Mode), "acme")
		previousCAFile := cfg.Server.TLS.CAFile
		result, err := ensureOwnerTLS(&cfg, runtimeDir)
		if err != nil {
			return fmt.Errorf("准备本地 TLS 证书失败: %w", err)
		}
		if wasACME {
			logger.Printf("[证书] 已从公网 ACME 迁移为本地 CA，不再需要 TCP 80")
		}
		if previousCAFile != "" && !strings.EqualFold(previousCAFile, result.CAFile) {
			logger.Printf("[证书] 旧证书算法兼容性不足，已保留旧文件并迁移到 ECDSA P-256")
		}
		logger.Printf("[证书] 本地 CA 已就绪 domain=%s expires=%s renewed=%t", cfg.Server.TLS.Domain, result.NotAfter.Format(time.RFC3339), result.Renewed)
		if err := config.SaveConfig(*configPath, cfg); err != nil {
			return err
		}
		if err := config.SaveProfile(ownerProfilePath, cfg.ClientProfile()); err != nil {
			if *initOwner {
				return err
			}
			logger.Printf("[配置] 自动更新客户端 profile 失败 path=%s error=%q", ownerProfilePath, err)
		} else {
			logger.Printf("[配置] 客户端 profile 已更新 path=%s version=%d", ownerProfilePath, config.ProfileVersion)
		}
	}

	if *initOwner {
		publicKey, err := sshtunnel.EnsurePrivateKey(cfg.Relay.PrivateKeyPath)
		if err != nil {
			return fmt.Errorf("生成 SSH 隧道密钥失败: %w", err)
		}
		if err := os.WriteFile(cfg.Relay.PrivateKeyPath+".pub", []byte(publicKey+"\n"), 0o600); err != nil {
			return err
		}
		if err := config.SaveConfig(*configPath, cfg); err != nil {
			return err
		}
		fmt.Printf("\n服务端模式已初始化。\n客户端配置: %s\nSSH 公钥（需要加入云服务器专用用户的 authorized_keys）:\n%s\n\n", ownerProfilePath, publicKey)
		logger.Printf("[配置] 混合服务端模式已初始化 profile=%s", ownerProfilePath)
	}

	if strings.TrimSpace(*exportProfile) != "" {
		if cfg.Server.AccessToken == "" {
			return errors.New("当前不是服务端配置，请先运行 --init-owner")
		}
		if err := config.SaveProfile(*exportProfile, cfg.ClientProfile()); err != nil {
			return err
		}
		fmt.Printf("客户端配置已导出: %s\n", *exportProfile)
	}

	extensionDir, err := config.DefaultExtensionDir()
	if err != nil {
		return err
	}
	if err := embedded.ExtractExtension(extensionDir); err != nil {
		logger.Printf("[扩展] 释放内置扩展失败 error=%q", err)
	} else {
		logger.Printf("[扩展] 浏览器扩展目录 path=%s", extensionDir)
	}

	if *installApp {
		installedPath, err := install.Install(*enableAutostart)
		if err != nil {
			return fmt.Errorf("安装失败: %w", err)
		}
		fmt.Printf("VideoWithYou 已安装到: %s\n浏览器扩展目录: %s\n", installedPath, extensionDir)
		return nil
	}
	if *extractExtension || strings.TrimSpace(*exportProfile) != "" || *initOwner || strings.TrimSpace(*importProfile) != "" {
		fmt.Printf("浏览器扩展目录: %s\n", extensionDir)
		return nil
	}

	if err := config.SaveConfig(*configPath, cfg); err != nil {
		logger.Printf("[配置] 保存标准化配置失败 error=%q", err)
	}
	return runApplication(cfg, *configPath, logger)
}

func runApplication(cfg config.Config, configPath string, logger *log.Logger) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if cfg.Server.Enabled && strings.EqualFold(strings.TrimSpace(cfg.Server.TLS.Mode), "local_ca") {
		go renewOwnerTLS(ctx, cfg, logger)
	}

	var hosted *hostserver.Server
	serverStartError := ""
	if cfg.Server.Enabled {
		room := roomserver.New(roomserver.Config{
			AccessToken:     cfg.Server.AccessToken,
			ReconnectGrace:  time.Duration(cfg.Server.ReconnectGraceSec) * time.Second,
			HostIdleTimeout: time.Duration(cfg.Server.HostIdleTimeoutSec) * time.Second,
		}, logger)
		room.Start(ctx)
		mux := http.NewServeMux()
		mux.HandleFunc(cfg.Server.Path, room.HandleWS)
		hosted = hostserver.New(hostserver.Config{
			ListenAddress: cfg.Server.ListenAddress,
			TLS: hostserver.TLSConfig{
				Mode:        cfg.Server.TLS.Mode,
				Domain:      cfg.Server.TLS.Domain,
				Email:       cfg.Server.TLS.Email,
				CacheDir:    cfg.Server.TLS.CacheDir,
				HTTPAddress: cfg.Server.TLS.HTTPAddress,
				CertFile:    cfg.Server.TLS.CertFile,
				KeyFile:     cfg.Server.TLS.KeyFile,
			},
		}, mux, logger)
		if err := hosted.Start(ctx); err != nil {
			serverStartError = err.Error()
			logger.Printf("[服务端] Windows 内嵌服务启动不完整，客户端功能继续运行 error=%q", err)
		}
	}

	extensionHost := extws.NewHost(cfg.ExtListenAddr, cfg.ExtListenPath, logger)
	localClient := client.New(cfg, configPath, extensionHost, logger)
	localClient.Start(ctx)
	localClient.SetServerStatus(serverStartError)

	if hosted != nil {
		hosted.SetOnStatus(func(status hostserver.Status) {
			parts := make([]string, 0, 2)
			if status.DirectError != "" {
				parts = append(parts, status.DirectError)
			}
			if status.ChallengeError != "" {
				parts = append(parts, "ACME: "+status.ChallengeError)
			}
			localClient.SetServerStatus(strings.Join(parts, "; "))
		})
	}

	if cfg.Server.Enabled && cfg.Relay.Enabled && hosted != nil {
		publicKey, err := sshtunnel.EnsurePrivateKey(cfg.Relay.PrivateKeyPath)
		if err != nil {
			logger.Printf("[隧道] SSH 密钥不可用 error=%q", err)
			localClient.SetTunnelStatus(false, err.Error())
		} else {
			_, serverPort, splitErr := net.SplitHostPort(cfg.Server.ListenAddress)
			if splitErr != nil {
				logger.Printf("[隧道] 本机服务监听地址无效 address=%s error=%q", cfg.Server.ListenAddress, splitErr)
				localClient.SetTunnelStatus(false, splitErr.Error())
			} else {
				localTarget := net.JoinHostPort("::1", serverPort)
				_ = os.WriteFile(cfg.Relay.PrivateKeyPath+".pub", []byte(publicKey+"\n"), 0o600)
				tunnel := sshtunnel.New(sshtunnel.Config{
					Address:             cfg.Relay.SSHAddress,
					User:                cfg.Relay.SSHUser,
					PrivateKeyPath:      cfg.Relay.PrivateKeyPath,
					HostKeyPinPath:      cfg.Relay.HostKeyPinPath,
					RemoteListenAddress: cfg.Relay.RemoteListenAddress,
					ReconnectDelay:      time.Duration(cfg.Relay.ReconnectDelaySec) * time.Second,
				}, logger)
				tunnel.SetOnStatus(func(status sshtunnel.Status) {
					localClient.SetTunnelStatus(status.Connected, status.Error)
				})
				logger.Printf("[隧道] IPv4 云转发目标 target=%s", localTarget)
				tunnel.Start(ctx, func(tunnelCtx context.Context, listener net.Listener) error {
					return sshtunnel.ProxyListener(tunnelCtx, listener, "tcp6", localTarget, logger)
				})
			}
		}
	}

	logger.Printf("[启动] 所有可用组件已启动，按 Ctrl+C 退出")
	<-ctx.Done()
	logger.Printf("[退出] 正在安全停止")
	if hosted != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = hosted.Shutdown(shutdownCtx)
	}
	return nil
}

func ensureOwnerTLS(cfg *config.Config, runtimeDir string) (localcert.Result, error) {
	certDir := strings.TrimSpace(cfg.Server.TLS.CacheDir)
	if certDir == "" {
		certDir = filepath.Join(runtimeDir, "certs")
	}
	result, err := localcert.Ensure(localcert.Config{
		Domain:    cfg.Server.TLS.Domain,
		Directory: certDir,
		CAFile:    cfg.Server.TLS.CAFile,
		CAKeyFile: cfg.Server.TLS.CAKeyFile,
		CertFile:  cfg.Server.TLS.CertFile,
		KeyFile:   cfg.Server.TLS.KeyFile,
	})
	if errors.Is(err, localcert.ErrUnsupportedCAAlgorithm) {
		cfg.Server.TLS.CAFile = filepath.Join(certDir, "owner-ca-ecdsa.pem")
		cfg.Server.TLS.CAKeyFile = filepath.Join(certDir, "owner-ca-ecdsa-key.pem")
		cfg.Server.TLS.CertFile = filepath.Join(certDir, "server-ecdsa.pem")
		cfg.Server.TLS.KeyFile = filepath.Join(certDir, "server-ecdsa-key.pem")
		result, err = localcert.Ensure(localcert.Config{
			Domain:    cfg.Server.TLS.Domain,
			Directory: certDir,
			CAFile:    cfg.Server.TLS.CAFile,
			CAKeyFile: cfg.Server.TLS.CAKeyFile,
			CertFile:  cfg.Server.TLS.CertFile,
			KeyFile:   cfg.Server.TLS.KeyFile,
		})
	}
	if err != nil {
		return localcert.Result{}, err
	}
	cfg.Server.TLS.Mode = "local_ca"
	cfg.Server.TLS.CacheDir = certDir
	cfg.Server.TLS.HTTPAddress = ""
	cfg.Server.TLS.CAFile = result.CAFile
	cfg.Server.TLS.CAKeyFile = result.CAKeyFile
	cfg.Server.TLS.CertFile = result.CertFile
	cfg.Server.TLS.KeyFile = result.KeyFile
	cfg.Connection.TLSCAPEM = result.CAPEM
	return result, nil
}

func renewOwnerTLS(ctx context.Context, cfg config.Config, logger *log.Logger) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := localcert.Ensure(localcert.Config{
				Domain:    cfg.Server.TLS.Domain,
				Directory: cfg.Server.TLS.CacheDir,
				CAFile:    cfg.Server.TLS.CAFile,
				CAKeyFile: cfg.Server.TLS.CAKeyFile,
				CertFile:  cfg.Server.TLS.CertFile,
				KeyFile:   cfg.Server.TLS.KeyFile,
			})
			if err != nil {
				logger.Printf("[证书] 本地证书续期检查失败 error=%q", err)
				continue
			}
			if result.Renewed {
				logger.Printf("[证书] 本地服务端证书已自动续期 expires=%s", result.NotAfter.Format(time.RFC3339))
			}
		}
	}
}

func loadConfigResilient(path string, logger *log.Logger) (config.Config, error) {
	cfg, err := config.LoadConfig(path)
	if err == nil {
		return cfg, nil
	}
	if _, statErr := os.Stat(path); statErr == nil {
		backup := fmt.Sprintf("%s.bad-%s", path, time.Now().Format("20060102-150405"))
		if data, readErr := os.ReadFile(path); readErr == nil {
			if writeErr := os.WriteFile(backup, data, 0o600); writeErr == nil {
				logger.Printf("[配置] 损坏配置已备份 path=%s", backup)
			}
		}
	}
	logger.Printf("[配置] 加载失败，将使用安全默认配置 error=%q", err)
	cfg = config.DefaultConfig()
	if ensureErr := cfg.EnsureIdentity(); ensureErr != nil {
		return config.Config{}, ensureErr
	}
	if saveErr := config.SaveConfig(path, cfg); saveErr != nil {
		return config.Config{}, saveErr
	}
	return cfg, nil
}
