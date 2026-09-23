package entrypoint

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// statCPU describes one CPU core's usage percentage.
type statCPU struct {
	ID    int     `json:"id"`
	Model string  `json:"model"`
	Usage float64 `json:"usage_percent"`
}

// statMemory reports capacity for memory or swap, in bytes.
type statMemory struct {
	Total     uint64  `json:"total"`
	Used      uint64  `json:"used"`
	Available uint64  `json:"available"`
	UsagePct  float64 `json:"usage_percent"`
}

// statDisk describes a mounted filesystem's capacity.
type statDisk struct {
	Device     string  `json:"device"`
	MountPoint string  `json:"mount_point"`
	FsType     string  `json:"fs_type"`
	Total      uint64  `json:"total"`
	Used       uint64  `json:"used"`
	Available  uint64  `json:"available"`
	UsagePct   float64 `json:"usage_percent"`
}

// statNet describes a network interface's traffic counters.
type statNet struct {
	Name      string `json:"name"`
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	TxPackets uint64 `json:"tx_packets"`
	State     string `json:"state"`
}

// statProc describes a single process.
type statProc struct {
	PID     int     `json:"pid"`
	User    string  `json:"user"`
	CPU     float64 `json:"cpu_percent"`
	Mem     float64 `json:"mem_percent"`
	RSS     uint64  `json:"rss_bytes"`
	VMS     uint64  `json:"vms_bytes"`
	Command string  `json:"command"`
}

// statProcDetail describes a process together with its runtime state, used by
// the Process tab to inspect the server's own process and the focused
// processes (opencode, codex).
type statProcDetail struct {
	PID     int     `json:"pid"`
	PPID    int     `json:"ppid"`
	User    string  `json:"user"`
	State   string  `json:"state"`
	CPU     float64 `json:"cpu_percent"`
	Mem     float64 `json:"mem_percent"`
	RSS     uint64  `json:"rss_bytes"`
	VMS     uint64  `json:"vms_bytes"`
	Threads int     `json:"threads"`
	Elapsed uint64  `json:"elapsed_seconds"`
	Command string  `json:"command"`
}

// statsResponse is the payload of GET /api/stats.
type statsResponse struct {
	Hostname         string           `json:"hostname"`
	OS               string           `json:"os"`
	Uptime           uint64           `json:"uptime_seconds"`
	LoadAvg          [3]float64       `json:"load_avg"`
	CPUCores         int              `json:"cpu_cores"`
	CPU              []statCPU        `json:"cpu"`
	Memory           statMemory       `json:"memory"`
	Swap             statMemory       `json:"swap"`
	Disks            []statDisk       `json:"disks"`
	Network          []statNet        `json:"network"`
	Processes        []statProc       `json:"processes"`
	ProcessesByCPU   []statProc       `json:"processes_by_cpu"`
	SelfProcess      *statProcDetail  `json:"self_process"`
	FocusedProcesses []statProcDetail `json:"focused_processes"`
	CollectedAt      time.Time        `json:"collected_at"`
}

func readProcLine(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func getLoadAvg() [3]float64 {
	var l [3]float64
	fields := strings.Fields(readProcLine("/proc/loadavg"))
	for i := 0; i < 3 && i < len(fields); i++ {
		if v, err := strconv.ParseFloat(fields[i], 64); err == nil {
			l[i] = v
		}
	}
	return l
}

func getUptime() uint64 {
	fields := strings.Fields(readProcLine("/proc/uptime"))
	if len(fields) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return uint64(v)
}

// cpuTimes is a snapshot of one CPU core's tick counters.
type cpuTimes struct {
	user, nice, system, idle, iowait, irq, softirq, steal uint64
}

// readCpuStats reads per-core tick counters from /proc/stat.
func readCpuStats() []cpuTimes {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil
	}
	var times []cpuTimes
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	for scan.Scan() {
		fields := strings.Fields(scan.Text())
		if len(fields) < 2 || !strings.HasPrefix(fields[0], "cpu") || fields[0] == "cpu" {
			// Skip the aggregate "cpu" line; only per-core lines are used.
			continue
		}
		var ct cpuTimes
		vals := make([]uint64, 0, len(fields)-1)
		for _, f := range fields[1:] {
			v, _ := strconv.ParseUint(f, 10, 64)
			vals = append(vals, v)
		}
		get := func(i int) uint64 {
			if i < len(vals) {
				return vals[i]
			}
			return 0
		}
		ct.user, ct.nice = get(0), get(1)
		ct.system, ct.idle = get(2), get(3)
		ct.iowait, ct.irq = get(4), get(5)
		ct.softirq, ct.steal = get(6), get(7)
		times = append(times, ct)
	}
	return times
}

func (c cpuTimes) total() uint64 {
	return c.user + c.nice + c.system + c.idle + c.iowait + c.irq + c.softirq + c.steal
}

func (c cpuTimes) busyTicks() uint64 { return c.total() - c.idle - c.iowait }

// usage computes per-core usage percentage between two snapshots.
func usage(prev, cur cpuTimes) float64 {
	total := cur.total() - prev.total()
	if total == 0 {
		return 0
	}
	busy := cur.busyTicks() - prev.busyTicks()
	return 100 * float64(busy) / float64(total)
}

func cpuModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	for scan.Scan() {
		if after, ok := strings.CutPrefix(scan.Text(), "model name"); ok {
			if i := strings.IndexByte(after, ':'); i >= 0 {
				return strings.TrimSpace(after[i+1:])
			}
		}
	}
	return ""
}

// readMemInfo parses /proc/meminfo into a key/value map (values in kibibytes).
func readMemInfo() map[string]uint64 {
	out := make(map[string]uint64)
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return out
	}
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	for scan.Scan() {
		fields := strings.Fields(scan.Text())
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			out[strings.TrimSuffix(fields[0], ":")] = v
		}
	}
	return out
}

func memoryStats(mem map[string]uint64) statMemory {
	kb := func(key string) uint64 { return mem[key] * 1024 }
	total := kb("MemTotal")
	available := kb("MemAvailable")
	used := uint64(0)
	if total > available {
		used = total - available
	}
	return statMemory{Total: total, Used: used, Available: available, UsagePct: pct(used, total)}
}

func swapStats(mem map[string]uint64) statMemory {
	kb := func(key string) uint64 { return mem[key] * 1024 }
	total := kb("SwapTotal")
	free := kb("SwapFree")
	used := uint64(0)
	if total > free {
		used = total - free
	}
	return statMemory{Total: total, Used: used, Available: free, UsagePct: pct(used, total)}
}

func pct(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(used) / float64(total)
}

func osRelease() string {
	if v := readProcLine("/proc/version"); v != "" {
		return v
	}
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if after, ok := strings.CutPrefix(strings.TrimSpace(line), "PRETTY_NAME="); ok {
				return strings.Trim(strings.TrimSpace(after), `"`)
			}
		}
	}
	return ""
}

func isPseudoFs(device, fsType string) bool {
	if strings.HasPrefix(device, "/") && !strings.HasPrefix(device, "/dev/") {
		return true
	}
	switch fsType {
	case "proc", "sysfs", "devtmpfs", "devpts", "tmpfs", "cgroup", "cgroup2",
		"overlay", "squashfs", "securityfs", "debugfs", "tracefs", "fusectl",
		"configfs", "pstore", "autofs", "hugetlbfs", "mqueue", "bpf", "binfmt_misc",
		"nfsd", "rpc_pipefs", "fuse.gvfsd-fuse", "nsfs":
		return true
	}
	return false
}

// diskStats returns filesystem usage for physical mounts.
func diskStats() []statDisk {
	var out []statDisk
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return out
	}
	seen := make(map[uint64]bool)
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	for scan.Scan() {
		fields := strings.Fields(scan.Text())
		if len(fields) < 3 || isPseudoFs(fields[0], fields[2]) {
			continue
		}
		device, mountPoint, fsType := fields[0], fields[1], fields[2]
		var st syscall.Statfs_t
		if err := syscall.Statfs(mountPoint, &st); err != nil {
			continue
		}
		key := uint64(st.Frsize) * st.Blocks
		if seen[key] {
			continue
		}
		seen[key] = true
		total := uint64(st.Frsize) * st.Blocks
		free := uint64(st.Frsize) * st.Bfree
		available := uint64(st.Frsize) * st.Bavail
		used := uint64(0)
		if total > free {
			used = total - free
		}
		out = append(out, statDisk{
			Device:     device,
			MountPoint: mountPoint,
			FsType:     fsType,
			Total:      total,
			Used:       used,
			Available:  available,
			UsagePct:   pct(used, total),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MountPoint < out[j].MountPoint })
	return out
}

// netStats reads per-interface traffic counters from /proc/net/dev.
func netStats() []statNet {
	var out []statNet
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		// Skip virtual and loopback interfaces (veth pairs, docker bridges,
		// libvirt NAT, etc.) that would clutter the overview.
		if isVirtualInterface(name) {
			continue
		}
		state := netState(name)
		if state == "unknown" {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}
		toUint := func(s string) uint64 {
			v, _ := strconv.ParseUint(s, 10, 64)
			return v
		}
		out = append(out, statNet{
			Name:      name,
			RxBytes:   toUint(fields[0]),
			RxPackets: toUint(fields[1]),
			TxBytes:   toUint(fields[8]),
			TxPackets: toUint(fields[9]),
			State:     state,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func netState(name string) string {
	st, err := os.ReadFile(filepath.Join("/sys/class/net", name, "operstate"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(st))
}

// isVirtualInterface reports whether an interface is virtual (bridges, veth
// pairs, docker/libvirt NAT) rather than a physical link.
func isVirtualInterface(name string) bool {
	switch {
	case name == "lo",
		strings.HasPrefix(name, "veth"),
		strings.HasPrefix(name, "docker"),
		strings.HasPrefix(name, "br-"),
		strings.HasPrefix(name, "virbr"),
		strings.HasPrefix(name, "vnet"),
		strings.HasPrefix(name, "vnic"),
		strings.HasPrefix(name, "vmnet"):
		return true
	}
	return false
}

// procStat holds the fields needed from /proc/<pid>/stat.
type procStat struct {
	pid       int
	comm      string
	state     string
	ppid      int
	utime     uint64
	stime     uint64
	rss       uint64 // in pages
	vms       uint64
	threads   int
	starttime uint64 // in clock ticks since boot
}

// parseProcStat parses /proc/<pid>/stat. comm may contain spaces or
// parentheses, so the closing paren locates the start of the remaining
// fields. After the closing paren the first field is the process state, so
// the proc(5) field N lives at index N-3 of `rest`: utime/stime are fields
// 14/15, num_threads is 20, starttime is 22, vsize is 23 and rss is 24.
func parseProcStat(s string) procStat {
	open := strings.Index(s, "(")
	end := strings.LastIndex(s, ")")
	if open < 0 || end < 0 || end <= open {
		return procStat{}
	}
	ps := procStat{comm: s[open+1 : end]}
	if pf := strings.Fields(s[:open]); len(pf) > 0 {
		ps.pid = atoi(pf[0])
	}
	rest := strings.Fields(s[end+1:])
	num := func(i int) uint64 {
		if i >= 0 && i < len(rest) {
			v, _ := strconv.ParseUint(rest[i], 10, 64)
			return v
		}
		return 0
	}
	numInt := func(i int) int {
		if i >= 0 && i < len(rest) {
			v, _ := strconv.Atoi(rest[i])
			return v
		}
		return 0
	}
	if len(rest) > 0 {
		ps.state = rest[0]
	}
	ps.ppid = numInt(1)
	ps.utime, ps.stime = num(11), num(12)
	ps.threads = numInt(17)
	ps.starttime = num(19)
	ps.vms, ps.rss = num(20), num(21)
	return ps
}

func processUser(pid string) string {
	data, err := os.ReadFile(filepath.Join("/proc", pid, "status"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if after, ok := strings.CutPrefix(line, "Uid:"); ok {
			fields := strings.Fields(after)
			if len(fields) == 0 {
				return ""
			}
			uid, err := strconv.Atoi(fields[0])
			if err != nil {
				return ""
			}
			if name := lookupUid(uid); name != "" {
				return name
			}
			return fields[0]
		}
	}
	return ""
}

func lookupUid(uid int) string {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 3 {
			continue
		}
		id, err := strconv.Atoi(fields[2])
		if err != nil || id != uid {
			continue
		}
		return fields[0]
	}
	return ""
}

func atoi(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

// totalSysTicks returns the aggregate CPU tick counter from /proc/stat, which
// spans all cores and serves as the denominator for per-process CPU deltas.
func totalSysTicks() uint64 {
	data := readProcLine("/proc/stat")
	var total uint64
	fields := strings.Fields(data)
	if len(fields) > 0 && fields[0] == "cpu" {
		for _, f := range fields[1:] {
			v, _ := strconv.ParseUint(f, 10, 64)
			total += v
		}
	}
	return total
}

// procSnapshot captures the tick counters of every running process.
type procSnapshot map[int]procStat

// snapshotProcs reads /proc/<pid>/stat for all numeric process directories.
func snapshotProcs() procSnapshot {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	snap := make(procSnapshot)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		ps := parseProcStat(string(data))
		if ps.pid != pid {
			continue
		}
		snap[pid] = ps
	}
	return snap
}

// processStats lists every running process. prev and cur are process tick
// snapshots taken at the start and end of a sampling interval, and sysDelta is
// the total system tick delta over that interval (across all cores). CPU
// usage is normalized to one core (like top). The returned slice is sorted by
// resident memory.
func processStats(prev, cur procSnapshot, sysDelta, totalMem uint64) []statProc {
	pageSize := uint64(os.Getpagesize())

	procs := make([]statProc, 0, len(cur))
	for pid, ps := range cur {
		prevPs, ok := prev[pid]
		if !ok {
			prevPs = ps // new process since the previous sample: no delta
		}
		rss := ps.rss * pageSize
		memPct := 0.0
		if totalMem > 0 {
			memPct = 100 * float64(rss) / float64(totalMem)
		}
		cpuPct := 0.0
		if sysDelta > 0 {
			procDelta := (ps.utime + ps.stime) - (prevPs.utime + prevPs.stime)
			// A small interval can produce a negative delta for a clock-skewed
			// process; clamp it to zero.
			if procDelta > 0 {
				cpuPct = 100.0 * float64(procDelta) / float64(sysDelta) * float64(cpuCount())
			}
		}
		procs = append(procs, statProc{
			PID:     pid,
			User:    processUser(strconv.Itoa(pid)),
			RSS:     rss,
			VMS:     ps.vms,
			Mem:     memPct,
			CPU:     cpuPct,
			Command: ps.comm,
		})
	}
	sort.Slice(procs, func(i, j int) bool {
		if procs[i].RSS != procs[j].RSS {
			return procs[i].RSS > procs[j].RSS
		}
		return procs[i].PID < procs[j].PID
	})
	return procs
}

// procClockTicks is the USER_HZ value used by the tick counters in
// /proc/<pid>/stat. It is part of the Linux userspace ABI and is fixed at 100
// regardless of the kernel's internal HZ.
const procClockTicks = 100

// focusProcessNames are the process names surfaced on the Process tab in
// addition to the server's own process.
var focusProcessNames = []string{"opencode", "codex"}

// procCmdline reads the full command line of a process, falling back to the
// comm value when cmdline is unavailable (for example kernel threads).
func procCmdline(pid int, fallback string) string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err == nil {
		args := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
		if joined := strings.TrimSpace(strings.Join(args, " ")); joined != "" {
			return joined
		}
	}
	return fallback
}

// isFocusProcess reports whether a process matches one of the focused process
// names, checking both the comm and the full command line.
func isFocusProcess(comm, cmdline string) bool {
	comm = strings.ToLower(comm)
	cmdline = strings.ToLower(cmdline)
	for _, name := range focusProcessNames {
		if strings.Contains(comm, name) || strings.Contains(cmdline, name) {
			return true
		}
	}
	return false
}

// buildProcDetail computes the Process-tab representation of a single process.
// prev is the process's snapshot at the start of the sampling interval (equal
// to cur for processes that appeared during it).
func buildProcDetail(pid int, cur, prev procStat, sysDelta, totalMem, uptime uint64, command string) statProcDetail {
	rss := cur.rss * uint64(os.Getpagesize())
	memPct := 0.0
	if totalMem > 0 {
		memPct = 100 * float64(rss) / float64(totalMem)
	}
	cpuPct := 0.0
	if sysDelta > 0 {
		delta := (cur.utime + cur.stime) - (prev.utime + prev.stime)
		if delta > 0 {
			cpuPct = 100.0 * float64(delta) / float64(sysDelta) * float64(cpuCount())
		}
	}
	elapsed := uint64(0)
	if started := cur.starttime / procClockTicks; uptime > started {
		elapsed = uptime - started
	}
	return statProcDetail{
		PID:     pid,
		PPID:    cur.ppid,
		User:    processUser(strconv.Itoa(pid)),
		State:   cur.state,
		CPU:     cpuPct,
		Mem:     memPct,
		RSS:     rss,
		VMS:     cur.vms,
		Threads: cur.threads,
		Elapsed: elapsed,
		Command: command,
	}
}

// truncate keeps the first `limit` entries of a slice, or the whole slice when
// limit is not positive.
func truncate(procs []statProc, limit int) []statProc {
	if limit > 0 && len(procs) > limit {
		procs = procs[:limit]
	}
	return procs
}

// cpuCount returns the number of online CPUs (the aggregate /proc/stat "cpu"
// line is ignored, so the number of per-core lines is used).
func cpuCount() int {
	return len(readCpuStats())
}

// GetStats is the HTTP handler for GET /api/stats. It reports an
// instantaneous snapshot of the host's CPU, memory, disk, network and
// top processes.
func (s *Server) GetStats(w http.ResponseWriter, r *http.Request) {
	prev := readCpuStats()
	prevSysTicks := totalSysTicks()
	prevProcs := snapshotProcs()
	// Sample CPU usage over a short interval for an accurate measurement.
	time.Sleep(200 * time.Millisecond)
	cur := readCpuStats()
	curSysTicks := totalSysTicks()
	curProcs := snapshotProcs()

	sysDelta := uint64(0)
	if curSysTicks > prevSysTicks {
		sysDelta = curSysTicks - prevSysTicks
	}

	cores := make([]statCPU, 0, len(cur))
	model := cpuModel()
	if prev != nil && len(prev) == len(cur) {
		for i := range cur {
			cores = append(cores, statCPU{ID: i, Model: model, Usage: usage(prev[i], cur[i])})
		}
	}

	mem := readMemInfo()
	totalMem := mem["MemTotal"] * 1024
	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}

	allProcs := processStats(prevProcs, curProcs, sysDelta, totalMem)
	byMem := truncate(allProcs, 50)
	byCPU := make([]statProc, len(allProcs))
	copy(byCPU, allProcs)
	sort.Slice(byCPU, func(i, j int) bool {
		if byCPU[i].CPU != byCPU[j].CPU {
			return byCPU[i].CPU > byCPU[j].CPU
		}
		return byCPU[i].PID < byCPU[j].PID
	})
	byCPU = truncate(byCPU, 50)

	// Collect the runtime state of the server's own process and of the
	// focused processes (opencode, codex) for the Process tab. The focused
	// list is intentionally unbounded so every matching process is shown.
	uptime := getUptime()
	prevAt := func(pid int, cur procStat) procStat {
		if ps, ok := prevProcs[pid]; ok {
			return ps
		}
		return cur
	}

	selfPID := os.Getpid()
	var selfProcess *statProcDetail
	if ps, ok := curProcs[selfPID]; ok {
		detail := buildProcDetail(selfPID, ps, prevAt(selfPID, ps), sysDelta, totalMem, uptime, procCmdline(selfPID, ps.comm))
		selfProcess = &detail
	}

	focused := make([]statProcDetail, 0)
	for pid, ps := range curProcs {
		if pid == selfPID {
			continue
		}
		cmdline := procCmdline(pid, ps.comm)
		if !isFocusProcess(ps.comm, cmdline) {
			continue
		}
		focused = append(focused, buildProcDetail(pid, ps, prevAt(pid, ps), sysDelta, totalMem, uptime, cmdline))
	}
	sort.Slice(focused, func(i, j int) bool { return focused[i].PID < focused[j].PID })

	writeJSONResponse(w, http.StatusOK, statsResponse{
		Hostname:         hostname,
		OS:               osRelease(),
		Uptime:           uptime,
		LoadAvg:          getLoadAvg(),
		CPUCores:         len(cores),
		CPU:              cores,
		Memory:           memoryStats(mem),
		Swap:             swapStats(mem),
		Disks:            diskStats(),
		Network:          netStats(),
		Processes:        byMem,
		ProcessesByCPU:   byCPU,
		SelfProcess:      selfProcess,
		FocusedProcesses: focused,
		CollectedAt:      time.Now(),
	})
}
