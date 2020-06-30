package util

import (
	"regexp"
	"strconv"
	"strings"
	"fmt"
	"os"
	"encoding/json"
)

var (
	dayofmonth = map[string]int{
		"01": 31,
		"02": 29,
		"03": 31,
		"04": 30,
		"05": 31,
		"06": 30,
		"07": 31,
		"08": 31,
		"09": 30,
		"10": 31,
		"11": 30,
		"12": 31,
	}

	regex *regexp.Regexp
	area  map[string]string
)

func init() {
	regex, _ = regexp.Compile(`^\d{17}[\dxX]$`)
	fd, _ := os.Open("./area.json")
	decoder := json.NewDecoder(fd)
	decoder.Decode(&area)
}

func Identity(card string) (ok bool, addr, birth, sex string) {
	if regex.MatchString(card) {
		var y, m, d, s = "", "", "", ""
		sex = "男"
		w1 := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
		w2 := []string{"1", "0", "X", "9", "8", "7", "6", "5", "4", "3", "2"}

		sum := 0
		for i := range w1 {
			val, _ := strconv.Atoi(card[i : i+1])
			sum += w1[i] * val
		}

		md := sum % 11
		if w2[md] != strings.ToUpper(string(card[17])) {
			return false, addr, birth, sex
		}

		a := card[0:6]
		y = card[6:10]
		m = card[10:12]
		d = card[12:14]
		s = card[16:17] // 性别

		// 日期验证
		addr, ak := area[a]
		days, dk := dayofmonth[m]
		day, _ := strconv.Atoi(d)
		if !ak || !dk || day < 1 || day > days {
			return false, addr, birth, sex
		}

		val, _ := strconv.Atoi(s)
		if val%2 == 0 {
			sex = "女"
		}

		birth = fmt.Sprintf("%v 年 %v 月 %v 日", y, m, d)
		return true, addr, birth, sex
	}

	return false, addr, birth, sex
}
