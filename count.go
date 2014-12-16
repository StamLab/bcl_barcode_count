package main

import (
	"compress/gzip"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"sort"
)

const (
	readChunkSize = 40000
)

func clustersToBarcodeMap(inputs []chan byte) map[string]int {
	decode := [4]byte{'A', 'C', 'G', 'T'}
	tally := make(map[string]int)
	barcodeLength := len(inputs)
	for {
		barcode := make([]byte, barcodeLength)
		channelEmpty := false
		for i, c := range inputs {
			cluster, okay := <-c
			channelEmpty = !okay
			if cluster == 0 {
				barcode[i] = 'N'
			} else {
				barcode[i] = decode[cluster&0x3]
			}
		}
		if channelEmpty {
			break
		}
		tally[string(barcode)]++
	}

	return tally
}

func bcl_to_clusters(filenames []string, output chan byte) {

	for _, filename := range filenames {
		// TODO: Error check for real
		file, err := os.Open(filename)
		defer file.Close()
		if err != nil {
			panic(err)
		}

		reader, gzip_err := gzip.NewReader(file)
		defer reader.Close()
		if gzip_err != nil {
			panic(gzip_err)
		}

		data := make([]byte, 4)

		reader.Read(data)
		count := binary.LittleEndian.Uint32(data)

		clusters := make([]byte, readChunkSize)

		sum := 0
		for {
			bytes_read, read_err := reader.Read(clusters)
			sum += bytes_read
			if read_err != nil || bytes_read == 0 {
				break
			}
			for i := 0; i < bytes_read; i++ {
				output <- clusters[i]
			}
		}

		if int(count) != int(sum) {
			panic(fmt.Sprintf("Expected %d clusters, found %d", count, sum))
		}

		reader.Close()
	}

	close(output)
}

type Pair struct {
	Key   string
	Value int
}

// A slice of Pairs that implements sort.Interface to sort by Value.
type PairList []Pair

func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }

// A function to turn a map into a PairList, then sort and return it.
func sortMapByValueDescending(m map[string]int) PairList {
	p := make(PairList, len(m))
	i := 0
	for k, v := range m {
		p[i] = Pair{k, v}
		i++
	}
	sort.Sort(sort.Reverse(p))
	return p
}

func reportOnFileGroups(fileGroups [][]string) map[string]int {

	comms := make([]chan byte, len(fileGroups))
	for i, files := range fileGroups {
		comms[i] = make(chan byte, readChunkSize)
		go bcl_to_clusters(files, comms[i])
	}

	tally := clustersToBarcodeMap(comms)

	return tally
}

func main() {

	currentDir, _ := os.Getwd()

	next_seq := flag.Bool("nextseq", false, "This is a NextSeq 500 flowcell")
	hi_seq := flag.Bool("hiseq", false, "This is a HiSeq flowcell")
	base_dir := flag.String("base", currentDir, "The base directory of the flowcell")
	mask := flag.String("mask", currentDir, "The bases mask to use for the flowcell")

	flag.Parse()

	maskToIndices(*mask)

	var fileGroups [][]string
	if *next_seq {
		fileGroups = getNextSeqFiles(*mask, *base_dir)
		tally := reportOnFileGroups(fileGroups)
		for _, pair := range sortMapByValueDescending(tally) {
			fmt.Println(pair.Key, pair.Value)
		}
		return
	}

	if *hi_seq {
		lanes := getHiSeqFiles(*mask, *base_dir)
		for l, fileGroups := range lanes {
			fmt.Printf("----Lane %d-----\n", l+1)
			tally := reportOnFileGroups(fileGroups)
			for _, pair := range sortMapByValueDescending(tally) {
				fmt.Println(pair.Key, pair.Value)
			}

		}

		return
	}

}
