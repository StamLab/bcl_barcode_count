// This is a generated file! Please edit source .ksy file and use kaitai-struct-compiler to rebuild

import "github.com/kaitai-io/kaitai_struct_go_runtime/kaitai"


type Bcl_Base int
const (
	Bcl_Base__A Bcl_Base = 0
	Bcl_Base__C Bcl_Base = 1
	Bcl_Base__G Bcl_Base = 2
	Bcl_Base__T Bcl_Base = 3
)
type Bcl struct {
	Header *Bcl_Header
	Clusters []*Bcl_Cluster
	_io *kaitai.Stream
	_root *Bcl
	_parent interface{}
}
func NewBcl() *Bcl {
	return &Bcl{
	}
}

func (this *Bcl) Read(io *kaitai.Stream, parent interface{}, root *Bcl) (err error) {
	this._io = io
	this._parent = parent
	this._root = root

	tmp1 := NewBcl_Header()
	err = tmp1.Read(this._io, this, this._root)
	if err != nil {
		return err
	}
	this.Header = tmp1
	for i := 1;; i++ {
		tmp2, err := this._io.EOF()
		if err != nil {
			return err
		}
		if tmp2 {
			break
		}
		tmp3 := NewBcl_Cluster()
		err = tmp3.Read(this._io, this, this._root)
		if err != nil {
			return err
		}
		this.Clusters = append(this.Clusters, tmp3)
	}
	return err
}
type Bcl_Header struct {
	ClusterCount uint32
	_io *kaitai.Stream
	_root *Bcl
	_parent *Bcl
}
func NewBcl_Header() *Bcl_Header {
	return &Bcl_Header{
	}
}

func (this *Bcl_Header) Read(io *kaitai.Stream, parent *Bcl, root *Bcl) (err error) {
	this._io = io
	this._parent = parent
	this._root = root

	tmp4, err := this._io.ReadU4le()
	if err != nil {
		return err
	}
	this.ClusterCount = uint32(tmp4)
	return err
}
type Bcl_Cluster struct {
	Base Bcl_Base
	Qual uint64
	_io *kaitai.Stream
	_root *Bcl
	_parent *Bcl
}
func NewBcl_Cluster() *Bcl_Cluster {
	return &Bcl_Cluster{
	}
}

func (this *Bcl_Cluster) Read(io *kaitai.Stream, parent *Bcl, root *Bcl) (err error) {
	this._io = io
	this._parent = parent
	this._root = root

	tmp5, err := this._io.ReadBitsIntBe(2)
	if err != nil {
		return err
	}
	this.Base = Bcl_Base(tmp5)
	tmp6, err := this._io.ReadBitsIntBe(6)
	if err != nil {
		return err
	}
	this.Qual = tmp6
	return err
}
