package main

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

const (
	readChunkSize      = 40000  // How many reads to grab at a time
	maxChunksInChannel = 100    // How many chunks to hold at once
	outputThreshold    = 100000 // Don't report barcodes with fewer clusters than this
)

func Min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func basesToBarcodeMap(barcodeChan chan []string, filterChan chan []byte) map[string]Count {
	tally := make(map[string]Count)

	for {
		barcodes, barcodeOpen := <-barcodeChan
		filters, filterOpen := <-filterChan

		for i := 0; i < len(barcodes); i++ {
			bc := string(barcodes[i])
			count := tally[bc]
			count.Total++
			count.Pass += int(filters[i])
			tally[bc] = count
		}

		if !barcodeOpen || !filterOpen {
			break
		}
	}

	return tally
}

func syncChannels(barcodeIn chan []string, filterIn chan []byte, barcodeOut chan []string, filterOut chan []byte) {
	defer close(barcodeOut)
	defer close(filterOut)

	var barcodes []string
	var filters []byte
	barcodeOpen := true
	filterOpen := true
	for {
		if len(barcodes) == 0 {
			barcodes, barcodeOpen = <-barcodeIn
		}
		if len(filters) == 0 {
			filters, filterOpen = <-filterIn
		}
		sendSize := Min(len(barcodes), len(filters))

		barcodeOut <- barcodes[:sendSize]
		filterOut <- filters[:sendSize]

		barcodes = barcodes[sendSize:]
		filters = filters[sendSize:]

		if !barcodeOpen || !filterOpen {
			break
		}
	}
}

func basesToBarcodes(inputs []chan []byte, output chan []string) {
	barcodeLength := len(inputs)
	channelEmpty := false
	defer close(output)

	for {
		barcodes := make([][]byte, readChunkSize)
		for cluster_idx := 0; cluster_idx < readChunkSize; cluster_idx++ {
			barcodes[cluster_idx] = make([]byte, barcodeLength)
		}

		numClusters := 0
		// Transpose bases into barcodes
		for i, c := range inputs {
			bases, okay := <-c
			channelEmpty = !okay
			numClusters = len(bases)
			for idx, base := range bases {
				barcodes[idx][i] = base
			}
		}
		if channelEmpty {
			break
		}

		barcodeStrings := make([]string, numClusters)
		for i := 0; i < numClusters; i++ {
			barcodeStrings[i] = string(barcodes[i])
		}
		output <- barcodeStrings
	}
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

func bclFileToClusters(filename string, output chan []byte) {

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
}

func bcl_to_clusters(filenames []string, output chan []byte) {

	for _, filename := range filenames {
		bclFileToClusters(filename, output)
	}

	close(output)
}

type Count struct {
	Total int
	Pass  int
}

type Pair struct {
	Key   string
	Value Count
}

func readFilterFiles(filenames []string, output chan []byte) {
	for _, filename := range filenames {
		readFilterFile(filename, output)
	}
	close(output)
}

func readFilterFile(filename string, output chan []byte) {
	file, _ := os.Open(filename)
	defer file.Close()

	debug_info := make([]byte, 12) // Refer to manual for spec

	file.Read(debug_info)

	for {
		filters := make([]byte, readChunkSize)
		bytes_read, read_err := file.Read(filters)
		if read_err != nil || bytes_read == 0 {
			break
		}
		output <- filters[:bytes_read]
	}
}

func reportOnFileGroups(fileGroups [][]string, filterFiles []string, output chan map[string]Count) {
	defer close(output)

	clusterComms := make([]chan []byte, len(fileGroups))
	baseComms := make([]chan []byte, len(fileGroups))
	filterComm := make(chan []byte)
	for i, files := range fileGroups {
		clusterComms[i] = make(chan []byte, maxChunksInChannel)
		baseComms[i] = make(chan []byte, maxChunksInChannel)
		go bcl_to_clusters(files, clusterComms[i])
		go clustersToBases(clusterComms[i], baseComms[i])
	}

	go readFilterFiles(filterFiles, filterComm)

	barcodeComm := make(chan []string)
	syncedBarcodeComm := make(chan []string)
	syncedFilterComm := make(chan []byte)

	go basesToBarcodes(baseComms, barcodeComm)
	go syncChannels(barcodeComm, filterComm, syncedBarcodeComm, syncedFilterComm)

	tally := basesToBarcodeMap(syncedBarcodeComm, syncedFilterComm)
	output <- tally
}

func printTallies(tallies []map[string]Count) {
	for _, tally := range tallies {
		for barcode, count := range tally {
			if count.Total < outputThreshold {
				delete(tally, barcode)
			}
		}
	}
	encode, err := json.MarshalIndent(tallies, "", "  ")
	if err != nil {
		panic(err)
	}
	os.Stdout.Write(encode)
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
	var filters [][]string
	if *next_seq {
		lanes, filters = getNextSeqFiles(*mask, *base_dir)
	} else if *hi_seq {
		lanes, filters = getHiSeqFiles(*mask, *base_dir)
	} else {
		panic("Must specify either --nextseq or --hiseq")
	}

	tallyComms := make([]chan map[string]Count, len(lanes))
	for l, fileGroups := range lanes {
		tallyComms[l] = make(chan map[string]Count)
		go reportOnFileGroups(fileGroups, filters[l], tallyComms[l])
	}

	tallies := make([]map[string]Count, len(lanes))
	for l := range lanes {
		tallies[l] = <-tallyComms[l]
	}

	printTallies(tallies)

}
