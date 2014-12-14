package main

import (
	"strings"
	"strconv"
	"regexp"
	"fmt"
	"path/filepath"
)

func maskToIndices(mask string) []int {
	regex := regexp.MustCompile("([yin])([0-9]+)?")
	parts := regex.FindAllStringSubmatch(strings.ToLower(mask), -1)
	cycles := []int{}

	currentCycle := 1
	for _, part := range parts {
		letter := part[1]
		count, _ := strconv.Atoi(part[2])
		if count < 1 {
			count = 1
		}

		if letter == "i" {
			for i:=0; i<count; i++ {
				cycles = append(cycles, currentCycle + i)
			}
		}
		currentCycle += count
	}

	return cycles
}

func getNextSeqFiles(mask string, basedir string) [][]string {
	cycles := maskToIndices(mask)
	files := make([][]string, len(cycles))
	for i, c := range cycles {
		cycle_file := fmt.Sprintf("%04d.bcl.bgzf", c)
		for l:=1;l<=4;l++ {
			lane := fmt.Sprintf("L%03d", l)
			file := filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, cycle_file)
			files[i] = append(files[i], file)

		}
	}
	return files
}
