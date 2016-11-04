package tcpassembly

import "fmt"

func (s *stream) RTTStat() string {
	if s.RttCount != 0 {
		first, max, min, total, recv := s.SYNRTT, s.MaxRTT, s.MinRTT, s.TotalRTT, s.RttCount
		rttStr := "RTT["

		if first < 5000 {
			rttStr += fmt.Sprintf("syn:%d(µs)/", first)
		} else {
			if first < 5*1000*1000 {
				rttStr += fmt.Sprintf("syn:%d(ms)/", first/1000)
			} else {
				rttStr += fmt.Sprintf("syn:%d(s)/", first/(1000*1000))
			}
		}

		if max < 5000 {
			rttStr += fmt.Sprintf("max:%d(µs)/", max)
		} else {
			if max < 5*1000*1000 {
				rttStr += fmt.Sprintf("max:%d(ms)/", max/1000)
			} else {
				rttStr += fmt.Sprintf("max:%d(s)/", max/(1000*1000))
			}
		}

		if min < 5000 {
			rttStr += fmt.Sprintf("min:%d(µs)/", min)
		} else {
			if min < 5*1000*1000 {
				rttStr += fmt.Sprintf("min:%d(ms)/", min/1000)
			} else {
				rttStr += fmt.Sprintf("min:%d(s)/", min/(1000*1000))
			}
		}

		if avg := (total / recv); avg < 5000 {
			rttStr += fmt.Sprintf("avg:%d(µs)|%d]", avg, recv)
		} else {
			if avg < 5*1000*1000 {
				rttStr += fmt.Sprintf("avg:%d(ms)|%d]", avg/1000, recv)
			} else {
				rttStr += fmt.Sprintf("avg:%d(s)|%d]", avg/(1000*1000), recv)
			}
		}
		return rttStr
	}
	return fmt.Sprintf("RTT[-1/-1/-1](µs)")
}

func (s *stream) BPStat(finish bool) string {
	speedStr := "B/P[tx:"

	var s2cbytes, olds2cbytes, s2cpackets, olds2cpackets, c2sbytes, oldc2sbytes, c2spackets, oldc2spackets int64

	s2cbytes, olds2cbytes, s2cpackets, olds2cpackets = s.s2c.Bytes, s.s2c.OldBytes, s.s2c.Packets, s.s2c.OldPackets
	s.s2c.OldPackets = s2cpackets
	s.s2c.OldBytes = s2cbytes

	if s.c2s != nil {
		c2sbytes, oldc2sbytes, c2spackets, oldc2spackets = s.c2s.Bytes, s.c2s.OldBytes, s.c2s.Packets, s.c2s.OldPackets
		s.c2s.OldPackets = c2spackets
		s.c2s.OldBytes = c2sbytes
	}

	if c2sbytes > 5*1024 {
		if c2sbytes > 5*1024*1024 {
			speedStr += fmt.Sprintf("%dMB/%d, rx:", c2sbytes/(1024*1024), c2spackets)
		} else {
			speedStr += fmt.Sprintf("%dKB/%d, rx:", c2sbytes/1024, c2spackets)
		}
	} else {
		speedStr += fmt.Sprintf("%dB/%d, rx:", c2sbytes, c2spackets)
	}

	if s2cbytes > 5*1024 {
		if s2cbytes > 5*1024*1024 {
			speedStr += fmt.Sprintf("%dMB/%d]", s2cbytes/(1024*1024), s2cpackets)
		} else {
			speedStr += fmt.Sprintf("%dKB/%d]", s2cbytes/1024, s2cpackets)
		}
	} else {
		speedStr += fmt.Sprintf("%dB/%d]", s2cbytes, s2cpackets)
	}
	if finish {
		return speedStr
	}

	if c2sspeed := (c2sbytes - oldc2sbytes) / 5; c2sspeed > 5*1024 {
		if c2sspeed > 5*1024*1024 {
			speedStr += fmt.Sprintf(" Bps/Pps[tx:%d(MB/s)/%.1f, rx:", c2sspeed/(1024*1024), float32(c2spackets-oldc2spackets)/5.0)
		} else {
			speedStr += fmt.Sprintf(" Bps/Pps[tx:%d(KB/s)/%.1f, rx:", c2sspeed/1024, float32(c2spackets-oldc2spackets)/5.0)
		}
	} else {
		speedStr += fmt.Sprintf(" Bps/Pps[tx:%d(B/s)/%.1f, rx:", c2sspeed, float32(c2spackets-oldc2spackets)/5.0)
	}

	if s2cspeed := (s2cbytes - olds2cbytes) / 5; s2cspeed > 5*1024 {
		if s2cspeed > 5*1024*1024 {
			speedStr += fmt.Sprintf("%d(MB/s)/%.1f]", s2cspeed/(1024*1024), float32(s2cpackets-olds2cpackets)/5.0)
		} else {
			speedStr += fmt.Sprintf("%d(KB/s)/%.1f]", s2cspeed/1024, float32(s2cpackets-olds2cpackets)/5.0)
		}
	} else {
		speedStr += fmt.Sprintf("%d(B/s)/%.1f]", s2cspeed, float32(s2cpackets-olds2cpackets)/5.0)
	}

	return speedStr
}
