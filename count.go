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
	readChunkSize      = 40000 // How many reads to grab at a time
	maxChunksInChannel = 100   // How many chunks to hold at once
)

func clustersToBarcodeMap(inputs []chan []byte) map[string]int {
	tally := make(map[string]int)
	barcodeLength := len(inputs)
	for {
		barcodes := make([][]byte, readChunkSize)
		for cluster_idx := 0; cluster_idx < readChunkSize; cluster_idx++ {
			barcodes[cluster_idx] = make([]byte, barcodeLength)
		}
		channelEmpty := false
		for i, c := range inputs {
			bases, okay := <-c
			channelEmpty = !okay
			for idx, base := range bases {
				barcodes[idx][i] = base
			}
		}
		if channelEmpty {
			break
		}
		for _, barcode := range barcodes {
			tally[string(barcode)]++
		}
	}

	return tally
}

func clustersToBases(input chan []byte, output chan []byte) {
	decode := [4]byte{'A', 'C', 'G', 'T'}
	for {
		clusters, okay := <-input

		bases := make([]byte, len(clusters))
		for i, cluster := range clusters {
			if cluster == 0 {
				bases[i] = 'N'
			} else {
				bases[i] = decode[cluster&0x3]
			}
		}
		output <- bases
		if !okay {
			break
		}

	}
	close(output)
}

func bcl_to_clusters(filenames []string, output chan []byte) {

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

		sum := 0
		for {
			clusters := make([]byte, readChunkSize)
			bytes_read, read_err := reader.Read(clusters)
			sum += bytes_read
			if read_err != nil || bytes_read == 0 {
				break
			}
			output <- clusters[:bytes_read]
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

	clusterComms := make([]chan []byte, len(fileGroups))
	baseComms := make([]chan []byte, len(fileGroups))
	for i, files := range fileGroups {
		clusterComms[i] = make(chan []byte, maxChunksInChannel)
		baseComms[i] = make(chan []byte, maxChunksInChannel)
		go bcl_to_clusters(files, clusterComms[i])
		go clustersToBases(clusterComms[i], baseComms[i])
	}

	tally := clustersToBarcodeMap(baseComms)

	return tally
}

func printTally(tally map[string]int) {
	for _, pair := range sortMapByValueDescending(tally) {
		fmt.Println(pair.Key, pair.Value)
	}
}

func main() {

	currentDir, _ := os.Getwd()

	next_seq := flag.Bool("nextseq", false, "This is a NextSeq 500 flowcell")
	hi_seq := flag.Bool("hiseq", false, "This is a HiSeq flowcell")
	base_dir := flag.String("base", currentDir, "The base directory of the flowcell")
	mask := flag.String("mask", currentDir, "The bases mask to use for the flowcell")

	flag.Parse()

	maskToIndices(*mask)

	var lanes [][][]string
	if *next_seq {
		lanes = getNextSeqFiles(*mask, *base_dir)
	} else if *hi_seq {
		lanes = getHiSeqFiles(*mask, *base_dir)
	} else {
		panic("Must specify either --nextseq or --hiseq")
	}

	for l, fileGroups := range lanes {
		fmt.Printf("----Lane %d-----\n", l+1)
		tally := reportOnFileGroups(fileGroups)
		printTally(tally)
	}

}
