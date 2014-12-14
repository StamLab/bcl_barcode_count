package main

import (
	"fmt"
	"compress/gzip"
	"os"
	"encoding/binary"
)


func bases_to_barcodes(bases [][]byte) []string{

	num_bases := len(bases)
	num_clusters := len(bases[0])

	barcodes := make([]string, num_clusters)

	//fmt.Println("num Bases/Clusters:", num_bases, num_clusters)

	// TODO: This has got to be really slow, right?
	for cluster:=0; cluster<num_clusters; cluster++ {

		barcode := make([]byte, num_bases)
		for i:=0; i<num_bases; i++ {
			barcode = append(barcode, bases[i][cluster] )
		}
		barcodes[cluster] = string(barcode)
	}

	return barcodes
}

func clusters_to_bases(clusters []byte) []byte{
	decode := [4]byte{'A', 'C', 'G', 'T'}
	bases := make([]byte, len(clusters))

	for i := 0; i < len(clusters); i++ {
		if (clusters[i] == 0) {
			bases[i] = 'N'
		} else {
			bases[i] = decode[ clusters[i] & 0x3 ]
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

func main() {

	cycles := [16]int{ 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52 }
	//cycles := [3]int{ 37, 38, 39}

	bases := make([][]byte, len(cycles))
	for i:=0; i<len(cycles); i++ {

		fmt.Println("File:", i)
		file := fmt.Sprintf("%04d.bcl.bgzf", cycles[i])
		clusters :=	bcl_to_clusters(file)
		bases[i] = clusters_to_bases( clusters )

	}

	barcodes := bases_to_barcodes(bases)

	//fmt.Println("Barcodes:", barcodes[0:10])
	for i :=0; i<len(barcodes); i++ {
		fmt.Println(barcodes[i])
	}
}
