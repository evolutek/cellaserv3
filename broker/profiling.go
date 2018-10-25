package broker

import (
	"flag"
	"os"
	"runtime/pprof"
)

var cpuprofile = flag.String("cpuprofile", "", "write CPU profile to file")

func (b *Broker) setupProfiling() {
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			b.logger.Fatal(err)
		}
		if err = pprof.StartCPUProfile(f); err != nil {
			b.logger.Error("Could not start CPU profiling:", err)
		}
	}
}

func (b *Broker) stopProfiling() {
	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}
}
