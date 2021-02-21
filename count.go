package main

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	// import "github.com/google/readahead"
	"github.com/google/readahead"
)

const (
	progVersion        = 0.3   // What version of the program to report
	readChunkSize      = 40000 // How many reads to grab at a time
	maxChunksInChannel = 100   // How many chunks to hold at once
)

// Min returns the minimum of two integers
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

// For each byte:
// if all 0: N
// 0bXXXXXX00 -> A
// 0bXXXXXX01 -> C
// 0bXXXXXX10 -> G
// 0bXXXXXX11 -> T
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

func assert(cond bool, reason string) {
	if !cond {
		panic(reason)
	}
}

type QvalMapInfo struct {
	binCount uint32
	binMap   map[uint32]uint32
}

type CblcHeaderStart struct {
	Version         uint16
	HeaderSize      uint32
	BitsPerBaseCall uint8
	BitsPerQScore   uint8
}

type TileInfo struct {
	TileNum          uint32
	ClusterCount     uint32
	UncompressedSize uint32
	CompressedSize   uint32
	//ExcludeNonPF     uint32 // Is this the right size??
}

type CblcHeader struct {
	start     CblcHeaderStart
	qvalInfo  QvalMapInfo
	tileCount uint32
	tiles     []TileInfo
}

func cbclFileToClusters(filename string, output chan []byte) {
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		panic(err)
	}

	binRead := func(dest interface{}) {
		err := binary.Read(file, binary.LittleEndian, dest)
		if err != nil {
			panic(err)
		}
	}

	// Read Header
	/*
		Bytes 0–1 Version number, current version is 1 unsigned 16 bits little endian integer
		Bytes 2–5 Header size unsigned 32 bits little endian integer
		Byte 6 Number of bits per base call unsigned
		Byte 7 Number of bits per q-score unsigned
		q-val mapping info
		Bytes 0–3 Number of bins (B), zero indicates no
		mapping
		B pairs of 4 byte values (if B > 0) {from, to}, {from, to}, {from, to} …
		from: quality score bin
		to: quality score
		Number of tile records unsigned 32 bits little endian integer
	*/

	header := CblcHeader{}
	binRead(&(header.start))

	header.qvalInfo.binMap = make(map[uint32]uint32)
	binRead(&header.qvalInfo.binCount)
	for i := uint32(0); i < header.qvalInfo.binCount; i++ {
		var from, to uint32
		binRead(&from)
		binRead(&to)
		header.qvalInfo.binMap[from] = to
	}

	binRead(&(header.tileCount))

	for i := uint32(0); i < header.tileCount; i++ {
		info := TileInfo{}
		binRead(&info)
		header.tiles = append(header.tiles, info)
	}

	assert(header.start.Version == 1, "cbcl version != 1")

	// Read the data!!!!
	offset := int64(header.start.HeaderSize)
	for _, tile := range header.tiles {
		// TODO: Work with variable bit lengths
		assert(header.start.BitsPerBaseCall == 2, "Error: not 2 bits per call")
		assert(header.start.BitsPerQScore == 2, "Error: not 2 bits per qscore")

		sr := io.NewSectionReader(file, offset, int64(tile.CompressedSize))
		err = cbclReadTile(sr, output)
		if err != nil {
			panic(fmt.Errorf("Error in %s: %s", filename, err))
		}

		offset += int64(tile.CompressedSize)
	}
}

// Slow and steady for v0
func cbclReadTile(sr io.Reader, output chan<- []byte) error {
	reader, gzipErr := gzip.NewReader(sr)
	if gzipErr != nil {
		return gzipErr
	}
	defer reader.Close()

	assert(readChunkSize%2 == 0, "read chunk size must be even")
	cc := readahead.NewReader("cbcl-reader-X", reader, readChunkSize/2, 1)
	defer cc.Close()

	for {
		clusters := make([]byte, readChunkSize)

		var b uint8
		for i := 0; i < len(clusters); i += 2 {
			err := binary.Read(cc, binary.LittleEndian, &b)
			if err == io.EOF {
				if i >= 2 {
					clusters = clusters[:i-2]
				} else {
					clusters = clusters[:i]
				}
				output <- clusters
				return nil
			}
			if err != nil {
				return err
			}

			// Algorithm:
			// For each of the 4-bit clusters, check:
			// Does quality == 0? if so -> 0
			// Otherwise -> send (128 | base), to make sure it's non-zero

			// Format:
			//   Q2B2Q1B1
			// 0b00000011 -> 1 + 2    = 3
			// 0b00001100 -> 8+4      = 12
			// 0b11000000 -> 128 + 64 = 196

			// Lower bits are first base
			if b&12 != 0 {
				clusters[i] = 128 | (b & 3)
			} else {
				clusters[i] = 0
			}
			// Upper bits are second base
			if b&196 != 0 {
				clusters[i+1] = 128 | ((b >> 4) & 3)
			} else {
				clusters[i+1] = 0
			}
		}
		output <- clusters
	}
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
		if strings.HasSuffix(filename, ".cbcl") {
			cbclFileToClusters(filename, output)
		} else {
			bclFileToClusters(filename, output)
		}
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

type Lane struct {
	LaneIndex int
	Total     int
	Pass      int
	Counts    map[string]Count
}

type Output struct {
	Sequencer string
	BaseDir   string
	Mask      string
	Lanes     []Lane
}

func printTallies(output Output, outputThreshold int) {
	for _, lane := range output.Lanes {
		for barcode, count := range lane.Counts {
			if count.Total < outputThreshold {
				delete(lane.Counts, barcode)
			}
		}
	}
	encode, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		panic(err)
	}
	os.Stdout.Write(encode)
}

func main() {

	currentDir, _ := os.Getwd()

	sayVersion := flag.Bool("version", false, "Output version info and exit")
	checkReady := flag.Bool("isready", false, "Only check if the files are ready to be processed")
	mini_seq := flag.Bool("miniseq", false, "This is a MiniSeq flowcell")
	next_seq := flag.Bool("nextseq", false, "This is a NextSeq 500 flowcell")
	hi_seq := flag.Bool("hiseq", false, "This is a HiSeq 2500 flowcell")
	hi_seq_4k := flag.Bool("hiseq4k", false, "This is a HiSeq 4000 flowcell")
	novaseq := flag.Bool("novaseq", false, "This is a NovaSeq flowcell")
	base_dir := flag.String("base", currentDir, "The base directory of the flowcell")
	mask := flag.String("mask", "y36,i8,i8,y36", "The bases mask to use for the flowcell")
	outputThreshold := flag.Int("threshold", 1000000, "Don't report below this threshold")

	flag.Parse()

	if *sayVersion {
		fmt.Println("Version: ", progVersion)
		os.Exit(0)
	}

	maskToIndices(*mask)

	var sequencer string
	var laneFiles [][][]string
	var filters [][]string
	var isReady bool

	if *next_seq {
		laneFiles, filters = getNextSeqFiles(*mask, *base_dir)
		isReady = isNextSeqReady(laneFiles, filters)
		sequencer = "NextSeq"
	} else if *hi_seq {
		isReady = isHiSeqReady(*mask, *base_dir)

		laneFiles, filters = getHiSeqFiles(*mask, *base_dir)
		sequencer = "HiSeq"
	} else if *hi_seq_4k {
		isReady = isHiSeq4kReady(*mask, *base_dir)

		laneFiles, filters = getHiSeq4kFiles(*mask, *base_dir)
		sequencer = "HiSeq 4000"

	} else if *mini_seq {
		laneFiles, filters = getMiniSeqFiles(*mask, *base_dir)
		isReady = isNextSeqReady(laneFiles, filters)
		sequencer = "MiniSeq"
	} else if *novaseq {
		laneFiles, filters = getNovaSeqFiles(*mask, *base_dir)
		isReady = isNovaSeqReady(*mask, *base_dir)
		sequencer = "NovaSeq"
	} else {
		panic("Must specify either --nextseq or --hiseq or --miniseq")
	}

	if isReady {
		if *checkReady {
			fmt.Println("Ready to process")
			os.Exit(0)
		}
	} else {
		fmt.Println("Not yet ready to process!")
		os.Exit(1)
	}

	tallyComms := make([]chan map[string]Count, len(laneFiles))
	for l, fileGroups := range laneFiles {
		tallyComms[l] = make(chan map[string]Count)
		go reportOnFileGroups(fileGroups, filters[l], tallyComms[l])
	}

	lanes := make([]Lane, len(laneFiles))
	for l := range lanes {
		tally := <-tallyComms[l]
		lane := Lane{l + 1, 0, 0, tally}
		for _, count := range tally {
			lane.Total += count.Total
			lane.Pass += count.Pass
		}
		lanes[l] = lane
	}

	output := Output{sequencer, *base_dir, *mask, lanes}
	printTallies(output, *outputThreshold)

}
