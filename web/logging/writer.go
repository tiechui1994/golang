package logging

import (
	"io"
	"sync"
	"time"
)

type logWriter struct {
	sync.Mutex
	writer io.Writer
}

func newLogWriter(wr io.Writer) *logWriter {
	return &logWriter{writer: wr}
}

// 写入消息, 不同的Writer导致消息写入不同的渠道
func (lg *logWriter) println(when time.Time, msg string) {
	lg.Lock()
	h, _, _ := formatTimeHeader(when)
	lg.writer.Write(append(append(h, msg...), '\n'))
	lg.Unlock()
}

const (
	y1 = `0123456789`
	y2 = `0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789`
	y3 = `0000000000111111111122222222223333333333444444444455555555556666666666777777777788888888889999999999`
	y4 = `0123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789`

	mo1 = `000000000111`
	mo2 = `123456789012`
	d1  = `0000000001111111111222222222233`
	d2  = `1234567890123456789012345678901`
	h1  = `000000000011111111112222`
	h2  = `012345678901234567890123`
	mi1 = `000000000011111111112222222222333333333344444444445555555555`
	mi2 = `012345678901234567890123456789012345678901234567890123456789`
	s1  = `000000000011111111112222222222333333333344444444445555555555`
	s2  = `012345678901234567890123456789012345678901234567890123456789`
	ns1 = `0123456789`
)

// 格式化时间 "2006/01/02 15:04:05.123 ", 高效格式化时间. 比使用Parse()快4倍
func formatTimeHeader(when time.Time) ([]byte, int, int) {
	year, month, day := when.Date()
	hour, min, sec := when.Clock()
	ns := when.Nanosecond() / 1000000
	//len("2006/01/02 15:04:05.123 ")==24
	var buf [24]byte

	buf[0] = y1[year/1000%10]
	buf[1] = y2[year/100]
	buf[2] = y3[year-year/100*100]
	buf[3] = y4[year-year/100*100]
	buf[4] = '/'
	buf[5] = mo1[month-1]
	buf[6] = mo2[month-1]
	buf[7] = '/'
	buf[8] = d1[day-1]
	buf[9] = d2[day-1]
	buf[10] = ' '
	buf[11] = h1[hour]
	buf[12] = h2[hour]
	buf[13] = ':'
	buf[14] = mi1[min]
	buf[15] = mi2[min]
	buf[16] = ':'
	buf[17] = s1[sec]
	buf[18] = s2[sec]
	buf[19] = '.'
	buf[20] = ns1[ns/100]
	buf[21] = ns1[ns%100/10]
	buf[22] = ns1[ns%10]

	buf[23] = ' '

	return buf[0:], day, hour
}
