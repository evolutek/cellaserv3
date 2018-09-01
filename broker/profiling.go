package broker

import (
	"flag"
	"os"
	"runtime/pprof"
)

var cpuprofile = flag.String("cpuprofile", "", "write CPU profile to file")

func setupProfiling() {
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		if err = pprof.StartCPUProfile(f); err != nil {
			log.Error("Could not start CPU profiling:", err)
		}
	}
}

func stopProfiling() {
	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}
}
