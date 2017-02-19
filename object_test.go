package s3

import "os"

func ExampleWalk() {
	walkFn := func(name string, info os.FileInfo) error {
		if info.IsDir() {
			return SkipDir
		}
		return nil
	}
	if err := Walk("https://s3-us-west-2.amazonaws.com/buckt_name/", walkFn, nil); err != nil {
		return
	}
}
