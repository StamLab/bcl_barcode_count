package main

import (
	"fmt"
	"os"
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

func getNextSeqFiles(mask string, basedir string) ([][][]string, [][]string) {
	cycles := maskToIndices(mask)
	files := make([][]string, len(cycles))
	filters := make([]string, NextSeqLanes)

	for l := 0; l < NextSeqLanes; l++ {
		lane := fmt.Sprintf("L%03d", l+1)
		for i, c := range cycles {
			cycleFile := fmt.Sprintf("%04d.bcl.bgzf", c)
			file := filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, cycleFile)
			files[i] = append(files[i], file)

		}
		filterFile := fmt.Sprintf("s_%d.filter", l+1)
		filters[l] = filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, filterFile)
	}
	return [][][]string{files}, [][]string{filters}
}

func getHiSeqFiles(mask string, basedir string) ([][][]string, [][]string) {
	cycles := maskToIndices(mask)
	files := make([][][]string, HiSeqLanes)
	filterFiles := make([][]string, HiSeqLanes)

	for l := 0; l < HiSeqLanes; l++ {
		lane := fmt.Sprintf("L%03d", l+1)
		files[l] = make([][]string, len(cycles))
		for i, c := range cycles {
			cycleDir := fmt.Sprintf("C%d.1", c)
			fileglob := filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, cycleDir, "s_*.bcl.gz")
			files[l][i], _ = filepath.Glob(fileglob)
		}
		filterGlob := filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, "s_*.filter")
		filterFiles[l], _ = filepath.Glob(filterGlob)
	}

	return files, filterFiles
}

func getHiSeq4kFiles(mask string, basedir string) ([][][]string, [][]string) {
	cycles := maskToIndices(mask)
	files := make([][][]string, HiSeqLanes)
	filterFiles := make([][]string, HiSeqLanes)

	for l := 0; l < HiSeqLanes; l++ {
		lane := fmt.Sprintf("L%03d", l+1)
		files[l] = make([][]string, len(cycles))
		for i, c := range cycles {
			cycleDir := fmt.Sprintf("C%d.1", c)
			fileglob := filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, cycleDir, "s_*.bcl.gz")
			files[l][i], _ = filepath.Glob(fileglob)
		}
		filterGlob := filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, "s_*.filter")
		filterFiles[l], _ = filepath.Glob(filterGlob)
	}

	return files, filterFiles
}

func isHiSeq4kReady(mask string, basedir string) bool {
	read_masks := strings.Split(strings.ToLower(mask), ",")
	last_index_read := 1
	for idx, m := range read_masks {
		if strings.Contains(m, "i") {
			last_index_read = idx + 1
		}
	}

	filename := fmt.Sprintf("RTARead%dComplete.txt", last_index_read)
	_, err := os.Stat(filepath.Join(basedir, filename))

	return (err == nil)
}

func isHiSeqReady(mask string, basedir string) bool {
	read_masks := strings.Split(strings.ToLower(mask), ",")
	last_index_read := 1
	for idx, m := range read_masks {
		if strings.Contains(m, "i") {
			last_index_read = idx + 1
		}
	}

	filename := fmt.Sprintf("Basecalling_Netcopy_complete_Read%d.txt", last_index_read)
	_, err := os.Stat(filepath.Join(basedir, filename))

	return (err == nil)
}

// This is more complicated
// For now, just check to see if all inputs exist.
func isNextSeqReady(bclFiles [][][]string, filterFiles [][]string) bool {

	for i := range bclFiles {
		for j := range bclFiles[i] {
			for _, file := range bclFiles[i][j] {
				stat, err := os.Stat(file)
				if err != nil || stat.Size() == 0 {
					return false
				}
			}
		}
	}

	for i := range filterFiles {
		for _, file := range filterFiles[i] {
			stat, err := os.Stat(file)
			if err != nil || stat.Size() == 0 {
				return false
			}
		}
	}

	return true
}
