package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/netip"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CTDirection struct {
	Src, Dst       netip.Addr
	Sport, Dport   uint16
	Packets, Bytes uint64
}

type CTLine struct {
	Orig, Reply CTDirection
}

type AcctData struct {
	Packets, Bytes uint64
}

type Recorder struct {
	mu   sync.Mutex
	data map[netip.Addr]AcctData
}

func (r *Recorder) Record(line CTLine) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := line.Orig.Src
	data := r.data[key]
	data.Packets += line.Orig.Packets + line.Reply.Packets
	data.Bytes += line.Orig.Bytes + line.Reply.Bytes
	r.data[key] = data
}

func (r *Recorder) reset() {
	r.data = make(map[netip.Addr]AcctData)
}

func (r *Recorder) Reset() {
	r.mu.Lock()
	r.reset()
	r.mu.Unlock()
}

type sortItem struct {
	Addr netip.Addr
	AcctData
}

func (r *Recorder) collect() []sortItem {
	items := make([]sortItem, 0, len(r.data))
	for k, v := range r.data {
		items = append(items, sortItem{k, v})
	}
	return items
}

func dumpItems(w io.Writer, items []sortItem) {
	now := time.Now()
	slices.SortFunc(items, func(a, b sortItem) int {
		// More bytes = sort first
		if a.Bytes != b.Bytes {
			return int(b.Bytes - a.Bytes)
		}
		if a.Packets != b.Packets {
			return int(b.Packets - a.Packets)
		}
		// Then sort by address
		return a.Addr.Compare(b.Addr)
	})
	buf := bufio.NewWriter(w)
	defer buf.Flush()
	fmt.Fprintf(buf, "Time: %s\n", now.Format(time.DateTime))
	for _, item := range items {
		fmt.Fprintf(buf, "  %40s %8d %12d\n", item.Addr.String(), item.Packets, item.Bytes)
	}
	fmt.Fprintln(buf)
}

func (r *Recorder) Dump(w io.Writer) {
	r.mu.Lock()
	items := r.collect()
	r.mu.Unlock()
	dumpItems(w, items)
}

func (r *Recorder) DumpAndReset(w io.Writer) {
	r.mu.Lock()
	items := r.collect()
	r.reset()
	r.mu.Unlock()
	dumpItems(w, items)
}

func ParseCTLine(s string) (CTLine, error) {
	var (
		line CTLine
		cur  *CTDirection
		err  error
	)
	for _, f := range strings.Fields(s) {
		parts := strings.SplitN(f, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "src":
			switch cur {
			case nil:
				cur = &line.Orig
			case &line.Orig:
				cur = &line.Reply
			default:
				return line, fmt.Errorf("unexpected src: %s", parts[1])
			}
			cur.Src, err = netip.ParseAddr(parts[1])
			if err != nil {
				return line, err
			}
		case "dst":
			cur.Dst, err = netip.ParseAddr(parts[1])
			if err != nil {
				return line, err
			}
		case "sport":
			value, err := strconv.ParseUint(parts[1], 10, 16)
			if err != nil {
				return line, err
			}
			cur.Sport = uint16(value)
		case "dport":
			value, err := strconv.ParseUint(parts[1], 10, 16)
			if err != nil {
				return line, err
			}
			cur.Dport = uint16(value)
		case "packets":
			cur.Packets, err = strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				return line, err
			}
		case "bytes":
			cur.Bytes, err = strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				return line, err
			}
		}
	}
	return line, nil
}

func sanityCheck() error {
	b, err := os.ReadFile("/proc/sys/net/netfilter/nf_conntrack_acct")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(b)) == "0" {
		return fmt.Errorf("nf_conntrack_acct is disabled")
	}
	return nil
}

// ctFilter determines whether a CTLine should be taken into accounting
func ctFilter(line CTLine) bool {
	if line.Orig.Packets+line.Reply.Packets < 10 {
		return false
	}
	if line.Orig.Bytes+line.Reply.Bytes < 1024 {
		return false
	}
	switch line.Orig.Dport {
	case 80, 443:
	default:
		return false
	}
	return true
}

func main() {
	var outFilename string
	flag.StringVar(&outFilename, "o", "conntrack.log", "output file")
	flag.Parse()
	outFile, err := os.OpenFile(outFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer outFile.Close()

	if err := sanityCheck(); err != nil {
		log.Println("Warning: sanity check failed:", err)
	}

	cmd := exec.Command("conntrack", "-E", "-e", "DESTROY", "-p", "tcp")
	reader, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err = cmd.Start(); err != nil {
		panic(err)
	}
	defer cmd.Wait()

	var (
		recorder Recorder
		last     time.Time = time.Now()
	)
	recorder.Reset()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		now := time.Now()
		line, err := ParseCTLine(scanner.Text())
		if err != nil {
			log.Println(err)
			continue
		}
		if !ctFilter(line) {
			continue
		}
		recorder.Record(line)
		if now.Minute() != last.Minute() {
			recorder.DumpAndReset(outFile)
		}
		last = now
	}
}
