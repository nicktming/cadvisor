package fs

type Context struct {
	Docker 			DockerContext
}

type DockerContext struct {
	Root 			string
	Driver 			string
	DriverStatus 		map[string]string
}


type FsType string

func (ft FsType) string() string {
	return string(ft)
}


type FsInfo interface {

}