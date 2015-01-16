Overview
========

This tool provides barcode quantification for Illumina sequencers from the BCL files created during sequencing. This quantification allows rapid identification of barcode errors as well as coarse feedback on overall sequencing quality.

Usage
=====

1. Navigate to the base flowcell directory.
2. `bcl_barcode_count --nextseq --mask=y36,i6n,y36 > barcodes.json`

Substitute `--nextseq`/`--hiseq` and the bases mask accordingly.


Installation
============

`go get github.com/StamLab/bcl_barcode_count`
