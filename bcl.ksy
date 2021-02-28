meta:
  id: bcl
  file-extension: bcl
  endian: le
seq:
  - id: header
    type: header
  - id: clusters
    type: cluster
    repeat: eos
types:
  header:
    seq:
      - id: cluster_count
        type: u4
  cluster:
    seq:
      - id: base
        enum: base
        type: b2
      - id: qual
        type: b6


enums:
  base:
    0: a
    1: c
    2: g
    3: t
