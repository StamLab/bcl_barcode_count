package main

import (
	"compress/gzip"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"sort"
)

func bases_to_barcodes(bases [][]byte) []string {

	num_bases := len(bases)
	num_clusters := len(bases[0])

	barcodes := make([]string, num_clusters)

	// TODO: This has got to be really slow, right?
	// Turns out it runs quick anyways.
	// Could be optimized but doesn't need it.
	for cluster_idx := range bases[0] {

		barcode := make([]byte, num_bases)
		for _, base := range bases {
			barcode = append(barcode, base[cluster_idx])
		}
		barcodes[cluster_idx] = string(barcode)
	}

	return barcodes
}

func clusters_to_bases(clusters []byte) []byte {
	decode := [4]byte{'A', 'C', 'G', 'T'}
	bases := make([]byte, len(clusters))

	for i, cluster := range clusters {
		if cluster == 0 {
			bases[i] = 'N'
		} else {
			bases[i] = decode[cluster&0x3]
		}
	}

	return bases
}

func bcl_to_clusters(filenames []string) []byte {

	allClusters := make([]byte, 1)

	for _, filename := range filenames {
		// TODO: Error check for real
		file, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		reader, gzip_err := gzip.NewReader(file)
		if gzip_err != nil {
			panic(gzip_err)
		}
		defer reader.Close()

		data := make([]byte, 4)
		reader.Read(data)
		count := binary.LittleEndian.Uint32(data)

		clusters := make([]byte, count)

		sum := 0
		for {
			bytes_read, read_err := reader.Read(clusters[sum:])
			sum += bytes_read
			if read_err != nil || bytes_read == 0 {
				break
			}

		}

		if int(count) != int(sum) {
			panic(fmt.Sprintf("Expected %d clusters, found %d", count, sum))
		}

		reader.Close()
		allClusters = append(allClusters, clusters...)
	}
	return (allClusters)

}

func tally_barcodes(barcodes []string) map[string]int {
	tally := make(map[string]int)

	for _, barcode := range barcodes {
		tally[barcode]++
	}

	return tally
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
	} else if *hi_seq {
		panic("Sorry, HiSeq processing NYI")
	}

	bases := make([][]byte, len(fileGroups))
	for i, files := range fileGroups {
		clusters := bcl_to_clusters(files)
		bases[i] = clusters_to_bases(clusters)
	}

	barcodes := bases_to_barcodes(bases)

	tally := tally_barcodes(barcodes)

	for _, pair := range sortMapByValueDescending(tally) {
		fmt.Println(pair.Key, pair.Value)
	}
}
