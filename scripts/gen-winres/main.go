// Command gen-winres generates cmd/drawgraph-gui/rsrc_windows_amd64.syso with the app icon,
// PerMonitorV2 DPI manifest, UTF-8 codepage, and VS_VERSION_INFO in pure Go.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
)

func main() {
	ver := flag.String("version", "0.1.0", "dotted semantic version")
	icoPath := flag.String("ico", "assets/drawgraph.ico", "multi-size .ico file")
	outPath := flag.String("out", "cmd/drawgraph-gui/rsrc_windows_amd64.syso", "output .syso path")
	flag.Parse()

	icoData, err := os.ReadFile(*icoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read ico: %v\n", err)
		os.Exit(1)
	}
	syso, err := buildSyso(*ver, icoData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build syso: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*outPath, syso, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write syso: %v\n", err)
		os.Exit(1)
	}
}

func buildSyso(ver string, ico []byte) ([]byte, error) {
	const (
		machineAMD64     uint16 = 0x8664
		relocAMD64ADDR32 uint16 = 0x0003
		rtIcon           uint16 = 3
		rtGroupIcon      uint16 = 14
		rtVersion        uint16 = 16
		rtManifest       uint16 = 24
		langENUS         uint16 = 0x0409
	)

	res := map[uint16]map[uint16][]byte{
		rtVersion:  {1: versionInfo(ver)},
		rtManifest: {1: []byte(manifestXML(ver))},
	}
	icons, group, err := parseICO(ico)
	if err != nil {
		return nil, err
	}
	res[rtIcon] = icons
	res[rtGroupIcon] = map[uint16][]byte{1: group}

	types := sortedKeys(res)
	dirSize := func(n int) int { return 16 + 8*n }
	size := dirSize(len(types))
	count := 0
	for _, t := range types {
		size += dirSize(len(res[t])) + len(res[t])*dirSize(1)
		count += len(res[t])
	}
	entriesAt := size
	dataAt := entriesAt + 16*count

	raw := make([]byte, dataAt)
	le := binary.LittleEndian
	dir := func(at, n int) { le.PutUint16(raw[at+14:], uint16(n)) }

	var relocs []uint32
	var data bytes.Buffer
	dir(0, len(types))
	next := dirSize(len(types))
	entry := entriesAt
	for i, t := range types {
		le.PutUint32(raw[16+8*i:], uint32(t))
		le.PutUint32(raw[16+8*i+4:], uint32(next)|0x80000000)
		ids := sortedKeys(res[t])
		idDir := next
		dir(idDir, len(ids))
		next += dirSize(len(ids))
		for j, id := range ids {
			le.PutUint32(raw[idDir+16+8*j:], uint32(id))
			le.PutUint32(raw[idDir+16+8*j+4:], uint32(next)|0x80000000)
			dir(next, 1)
			le.PutUint32(raw[next+16:], uint32(langENUS))
			le.PutUint32(raw[next+20:], uint32(entry))
			next += dirSize(1)
			for data.Len()%8 != 0 {
				data.WriteByte(0)
			}
			le.PutUint32(raw[entry:], uint32(dataAt+data.Len()))
			le.PutUint32(raw[entry+4:], uint32(len(res[t][id])))
			relocs = append(relocs, uint32(entry))
			entry += 16
			data.Write(res[t][id])
		}
	}
	raw = append(raw, data.Bytes()...)
	for len(raw)%4 != 0 {
		raw = append(raw, 0)
	}

	var out bytes.Buffer
	const headerSize, sectionHeaderSize = 20, 40
	rawAt := headerSize + sectionHeaderSize
	relocAt := rawAt + len(raw)
	symbolsAt := relocAt + 10*len(relocs)
	_ = binary.Write(&out, le, struct {
		Machine, Sections          uint16
		Time, Symbols, SymbolCount uint32
		OptSize, Characteristics   uint16
	}{machineAMD64, 1, 0, uint32(symbolsAt), 1, 0, 0})
	_ = binary.Write(&out, le, struct {
		Name                         [8]byte
		VSize, VAddr, RawSize, RawAt uint32
		RelocAt, LineAt              uint32
		Relocs, Lines                uint16
		Characteristics              uint32
	}{[8]byte{'.', 'r', 's', 'r', 'c'}, 0, 0, uint32(len(raw)), uint32(rawAt), uint32(relocAt), 0, uint16(len(relocs)), 0, 0x40000040})
	out.Write(raw)
	for _, r := range relocs {
		_ = binary.Write(&out, le, r)
		_ = binary.Write(&out, le, uint32(0))
		_ = binary.Write(&out, le, relocAMD64ADDR32)
	}
	out.Write([]byte{'.', 'r', 's', 'r', 'c', 0, 0, 0})
	_ = binary.Write(&out, le, uint32(0))
	_ = binary.Write(&out, le, uint16(1))
	_ = binary.Write(&out, le, uint16(0))
	out.Write([]byte{3, 0})
	_ = binary.Write(&out, le, uint32(4))
	return out.Bytes(), nil
}

func parseICO(ico []byte) (map[uint16][]byte, []byte, error) {
	if len(ico) < 6 {
		return nil, nil, fmt.Errorf("invalid ico")
	}
	le := binary.LittleEndian
	count := int(le.Uint16(ico[4:6]))
	if len(ico) < 6+16*count {
		return nil, nil, fmt.Errorf("truncated ico directory")
	}
	imgs := map[uint16][]byte{}
	var group bytes.Buffer
	_ = binary.Write(&group, le, [3]uint16{0, 1, uint16(count)})
	for i := 0; i < count; i++ {
		off := 6 + 16*i
		sz := le.Uint32(ico[off+8 : off+12])
		imgAt := le.Uint32(ico[off+12 : off+16])
		if int(imgAt+sz) > len(ico) {
			return nil, nil, fmt.Errorf("ico entry %d out of range", i)
		}
		id := uint16(i + 1)
		imgs[id] = ico[imgAt : imgAt+sz]
		group.Write(ico[off : off+12])
		_ = binary.Write(&group, le, id)
	}
	return imgs, group.Bytes(), nil
}

func manifestXML(ver string) string {
	v := parseVer(ver)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity type="win32" name="com.neko233.drawgraph" version="%d.%d.%d.%d" processorArchitecture="*"/>
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/>
    </dependentAssembly>
  </dependency>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>
  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware>
      <dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">PerMonitorV2</dpiAwareness>
      <activeCodePage xmlns="http://schemas.microsoft.com/SMI/2019/WindowsSettings">UTF-8</activeCodePage>
    </windowsSettings>
  </application>
</assembly>
`, v[0], v[1], v[2], v[3])
}

func versionInfo(ver string) []byte {
	v := parseVer(ver)
	fixed := make([]byte, 52)
	le := binary.LittleEndian
	le.PutUint32(fixed[0:], 0xFEEF04BD)
	le.PutUint32(fixed[4:], 0x00010000)
	le.PutUint32(fixed[8:], uint32(v[0])<<16|uint32(v[1]))
	le.PutUint32(fixed[12:], uint32(v[2])<<16|uint32(v[3]))
	copy(fixed[16:24], fixed[8:16])
	le.PutUint32(fixed[24:], 0x3F)
	le.PutUint32(fixed[32:], 0x00040004)
	le.PutUint32(fixed[36:], 1)

	var table []verNode
	for _, kv := range [][2]string{
		{"CompanyName", "neko233-com"},
		{"FileDescription", "DrawGraph"},
		{"FileVersion", ver},
		{"InternalName", "drawgraph-gui.exe"},
		{"LegalCopyright", "Copyright (c) 2026 neko233-com"},
		{"OriginalFilename", "drawgraph-gui.exe"},
		{"ProductName", "DrawGraph"},
		{"ProductVersion", ver},
		{"MyGoIdentifier", "com.neko233.drawgraph"},
	} {
		u16 := utf16.Encode([]rune(kv[1] + "\x00"))
		val := make([]byte, 2*len(u16))
		for i, u := range u16 {
			le.PutUint16(val[2*i:], u)
		}
		table = append(table, verNode{key: kv[0], text: true, value: val, valueLen: len(u16)})
	}
	root := verNode{
		key:      "VS_VERSION_INFO",
		value:    fixed,
		valueLen: len(fixed),
		children: []verNode{
			{key: "StringFileInfo", text: true, children: []verNode{{key: "040904b0", text: true, children: table}}},
			{key: "VarFileInfo", text: true, children: []verNode{{key: "Translation", value: []byte{0x09, 0x04, 0xB0, 0x04}, valueLen: 4}}},
		},
	}
	return root.encode()
}

type verNode struct {
	key      string
	text     bool
	value    []byte
	valueLen int
	children []verNode
}

func (n verNode) encode() []byte {
	b := make([]byte, 6)
	for _, u := range utf16.Encode([]rune(n.key + "\x00")) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	pad := func() {
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
	}
	pad()
	b = append(b, n.value...)
	for _, ch := range n.children {
		pad()
		b = append(b, ch.encode()...)
	}
	binary.LittleEndian.PutUint16(b[0:], uint16(len(b)))
	binary.LittleEndian.PutUint16(b[2:], uint16(n.valueLen))
	if n.text {
		binary.LittleEndian.PutUint16(b[4:], 1)
	}
	return b
}

func parseVer(v string) [4]uint16 {
	var out [4]uint16
	v, _, _ = strings.Cut(strings.TrimPrefix(v, "v"), "-")
	for i, part := range strings.SplitN(v, ".", 4) {
		n, _ := strconv.Atoi(part)
		out[i] = uint16(n)
	}
	return out
}

func sortedKeys[V any](m map[uint16]V) []uint16 {
	keys := make([]uint16, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
