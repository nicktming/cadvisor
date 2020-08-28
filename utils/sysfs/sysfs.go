package sysfs

type SysFs interface {

}


type realSysFs struct {}


func NewRealSysFs() SysFs {
	return &realSysFs{}
}