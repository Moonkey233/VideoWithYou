package ntp

func ComputeOffsetDelay(t1, t2, t3, t4 int64) (offset int64, delay int64) {
    delay = (t4 - t1) - (t3 - t2)
    offset = ((t2 - t1) + (t3 - t4)) / 2
    return offset, delay
}