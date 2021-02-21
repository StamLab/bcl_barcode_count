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
	MiniSeqLanes = 1
	NextSeqLanes = 4
	HiSeqLanes   = 8
)

// FileFinder helps you find files
type FileFinder interface {
	getFiles(mask string, basedir string) ([][][]string, [][]string)
	isReady(mask string, basedir string) bool
}

type finder struct {
	mask    string
	basedir string
}

// NextseqFileFinder returns the files for nextseq
type NextseqFileFinder struct {
	finder
}

func (n *NextseqFileFinder) getFiles() ([][][]string, [][]string) {
	return getNextSeqFiles(n.mask, n.basedir)
}

func (n *NextseqFileFinder) isReady() bool {
	return isNextSeqReady(n.getFiles())
}

func mustGlob(path string) (files []string) {
	files, err := filepath.Glob(path)
	if err != nil {
		panic(err)
	}
	return files
}

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

func getMiniSeqFiles(mask string, basedir string) ([][][]string, [][]string) {
	cycles := maskToIndices(mask)
	files := make([][]string, len(cycles))
	filters := make([]string, MiniSeqLanes)

	for l := 0; l < MiniSeqLanes; l++ {
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

// A SeqFileGroup contains all files you need to process a flowcell.
type SeqFileGroup struct {
	LaneFiles []LaneFileGroup
}

// A LaneFileGroup contains the files to process one lane of sequencing.
type LaneFileGroup struct {
	CycleFiles  []FileGroup
	FilterFiles FileGroup
}

// A FileGroup is a set of files. When read together, they should be concatenated.
type FileGroup struct {
	Files []string
}

// [][][]string is a nested list of file names
// []           : One entry for each lane  (e.g: L003)
//   []         : One entry for each cycle (e.g: C102)
//     []string : A list of files for a given lane/cycle combo

// [][]string is a list of filter files, one list per lane
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
			files[l][i] = mustGlob(fileglob)
		}
		filterGlob := filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, "s_*.filter")
		filterFiles[l] = mustGlob(filterGlob)
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
			files[l][i] = mustGlob(fileglob)
		}
		filterGlob := filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, "s_*.filter")
		filterFiles[l] = mustGlob(filterGlob)
	}

	return files, filterFiles
}

func getNovaSeqFiles(mask string, basedir string) ([][][]string, [][]string) {
	cycles := maskToIndices(mask)

	basecallDir := filepath.Join(basedir, "Data", "Intensities", "BaseCalls")
	laneDirGlob := filepath.Join(basecallDir, "L???")
	numLanes := len(mustGlob(laneDirGlob))

	files := make([][][]string, numLanes)
	filterFiles := make([][]string, numLanes)

	for l := 0; l < numLanes; l++ {
		lane := fmt.Sprintf("L%03d", l+1)
		files[l] = make([][]string, len(cycles))
		for i, c := range cycles {
			cycleDir := fmt.Sprintf("C%d.1", c)
			fileglob := filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, cycleDir, "*.cbcl")
			files[l][i] = mustGlob(fileglob)
		}
		filterGlob := filepath.Join(basedir, "Data", "Intensities", "BaseCalls", lane, "s_*.filter")
		filterFiles[l] = mustGlob(filterGlob)
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

func isNovaSeqReady(mask string, basedir string) bool {
	filename := "RTAComplete.txt"
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
