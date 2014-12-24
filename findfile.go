package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	NextSeqLanes = 4
	HiSeqLanes   = 8
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
			for i := 0; i < count; i++ {
				cycles = append(cycles, currentCycle+i)
			}
		}
		currentCycle += count
	}

	return cycles
}

func getNextSeqFiles(mask string, basedir string) [][][]string {
	cycles := maskToIndices(mask)
	files := make([][]string, len(cycles))
	for i, c := range cycles {
		cycleFile := fmt.Sprintf("%04d.bcl.bgzf", c)
		for l := 1; l <= NextSeqLanes; l++ {
			lane := fmt.Sprintf("L%03d", l)
			file := filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, cycleFile)
			files[i] = append(files[i], file)

		}
	}
	return [][][]string{files}
}

func getHiSeqFiles(mask string, basedir string) [][][]string {
	cycles := maskToIndices(mask)
	files := make([][][]string, HiSeqLanes)

	for l := 0; l < HiSeqLanes; l++ {
		lane := fmt.Sprintf("L%03d", l+1)
		files[l] = make([][]string, len(cycles))
		for i, c := range cycles {
			cycleDir := fmt.Sprintf("C%d.1", c)
			fileglob := filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, cycleDir, "s_*.bcl.gz")
			files[l][i], _ = filepath.Glob(fileglob)
		}
	}

	return files
}
