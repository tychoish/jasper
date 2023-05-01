package jasper

import (
	"testing"

	"github.com/tychoish/fun/assert/check"
)

func TestGetPids(t *testing.T) {
	log := "2018-10-03 21:55:21.478932+0000 0x16b Default 0x0 0 kernel: low swap: killing largest compressed process with pid 29670 (mongod) and size 1 MB"

	dmesg := "[11686.043647] Killed process 2603 (flasherav) total-vm:1498536kB, anon-rss:721784kB, file-rss:4228kB"

	check.True(dmesgContainsOOMKill(dmesg))

	pid, hasPid := getPidFromDmesg(dmesg)
	check.True(t, hasPid)
	check.Equal(t, pid, 2603)

	pid, hasPid = getPidFromLog(log)
	check.True(t, hasPid)
	check.Equal(t, pid, 29670)
}
