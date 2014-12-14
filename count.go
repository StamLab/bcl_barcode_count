package main

import (
	"fmt"
	"compress/gzip"
	"os"
	"encoding/binary"
	"flag"
)


func bases_to_barcodes(bases [][]byte) []string{

	num_bases := len(bases)
	num_clusters := len(bases[0])

	barcodes := make([]string, num_clusters)

	// TODO: This has got to be really slow, right?
	for cluster_idx := range bases[0] {

		barcode := make([]byte, num_bases)
		for _, base := range bases{
			barcode = append(barcode, base[cluster_idx] )
		}
		barcodes[cluster_idx] = string(barcode)
	}

	return barcodes
}

func clusters_to_bases(clusters []byte) []byte{
	decode := [4]byte{'A', 'C', 'G', 'T'}
	bases := make([]byte, len(clusters))

	for i, cluster := range clusters{
		if (cluster == 0) {
			bases[i] = 'N'
		} else {
			bases[i] = decode[ cluster & 0x3 ]
		}
	}

	return bases
}

func bcl_to_clusters(filename string) []byte{

	// TODO: Error check for real
	file, err := os.Open(filename)
	if (err != nil){
		panic(err)
	}
	defer file.Close()

	reader, gzip_err := gzip.NewReader(file)
	if (gzip_err != nil){
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
	return(clusters)
}

func tally_barcodes(barcodes []string) map[string]int {
	tally := make(map[string]int)

	for _, barcode := range barcodes{
		tally[barcode]++
	}

	return tally
}

func main() {

	flag.Parse()
	files := flag.Args()

	bases := make([][]byte, len(files))

	for i, file := range files {
		clusters :=	bcl_to_clusters(file)
		bases[i] = clusters_to_bases( clusters )
	}

	barcodes := bases_to_barcodes(bases)

	tally := tally_barcodes(barcodes)

	for barcode, count := range tally {
		fmt.Println(barcode, count)
	}
}
