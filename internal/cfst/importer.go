package cfst

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// MaxUploadSize 为上传文件的大小上限。
const MaxUploadSize = 64 << 20

// BadFileError 表示文件本身不可用（平台不匹配、无法执行、格式错误），对应 HTTP 400。
type BadFileError struct{ Msg string }

func (e *BadFileError) Error() string { return e.Msg }

func badFile(format string, a ...any) error { return &BadFileError{Msg: fmt.Sprintf(format, a...)} }

// ErrInstalling 表示已有安装在进行。
var ErrInstalling = errors.New("正在安装中，请稍候")

// ErrTaskActive 表示有任务排队或运行中，此时不能替换二进制。
var ErrTaskActive = errors.New("有任务正在执行或排队，请稍后再安装或导入")

// ImportResult 为导入结果。
type ImportResult struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// Candidate 为本机发现的可导入 cfst。
type Candidate struct {
	Path       string `json:"path"`
	Source     string `json:"source"` // datadir / path / bundled
	Version    string `json:"version"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	Compatible bool   `json:"compatible"`
	Message    string `json:"message,omitempty"`
}

// begin 占用安装标记，返回释放函数。
// 先置标记再检查任务：执行引擎先把状态写为 running 再调用 Run，Run 会拒绝安装中的状态，
// 两边总有一方能看到另一方，避免检查与替换之间有测速启动。
func (m *Manager) begin() (func(), error) {
	m.mu.Lock()
	if m.installing {
		m.mu.Unlock()
		return nil, ErrInstalling
	}
	m.installing = true
	m.mu.Unlock()
	done := func() {
		m.mu.Lock()
		m.installing = false
		m.mu.Unlock()
	}
	if m.Busy != nil && m.Busy() {
		done()
		return nil, ErrTaskActive
	}
	return done, nil
}

// isInstalling 返回是否正在安装。
func (m *Manager) isInstalling() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.installing
}

// ---------- 文件头识别 ----------

// DetectPlatform 从 ELF / Mach-O / PE 文件头识别目标 OS 与架构。
// Mach-O 通用二进制返回本机架构（若包含），否则返回第一个架构。
func DetectPlatform(r io.ReaderAt) (goos, goarch string, err error) {
	var magic [4]byte
	if _, err := r.ReadAt(magic[:], 0); err != nil {
		return "", "", badFile("不是可执行文件")
	}
	switch {
	case bytes.Equal(magic[:], []byte(elf.ELFMAG)):
		f, err := elf.NewFile(r)
		if err != nil {
			return "", "", badFile("ELF 文件头损坏: %v", err)
		}
		return elfOS(f.OSABI), elfArch(f), nil
	case magic[0] == 'M' && magic[1] == 'Z':
		f, err := pe.NewFile(r)
		if err != nil {
			return "", "", badFile("PE 文件头损坏: %v", err)
		}
		return "windows", peArch(f.Machine), nil
	}
	if ff, err := macho.NewFatFile(r); err == nil {
		var arches []string
		for _, a := range ff.Arches {
			arches = append(arches, machoArch(a.Cpu))
		}
		if slices.Contains(arches, runtime.GOARCH) {
			return "darwin", runtime.GOARCH, nil
		}
		if len(arches) > 0 {
			return "darwin", arches[0], nil
		}
	}
	if f, err := macho.NewFile(r); err == nil {
		return "darwin", machoArch(f.Cpu), nil
	}
	return "", "", badFile("不是可识别的可执行文件（支持 ELF / Mach-O / PE）")
}

func elfOS(abi elf.OSABI) string {
	switch abi {
	case elf.ELFOSABI_FREEBSD:
		return "freebsd"
	case elf.ELFOSABI_NETBSD:
		return "netbsd"
	case elf.ELFOSABI_OPENBSD:
		return "openbsd"
	}
	// Linux 程序通常标记为 SYSV（0）
	return "linux"
}

func elfArch(f *elf.File) string {
	le := f.Data == elf.ELFDATA2LSB
	is64 := f.Class == elf.ELFCLASS64
	switch f.Machine {
	case elf.EM_X86_64:
		return "amd64"
	case elf.EM_386:
		return "386"
	case elf.EM_AARCH64:
		return "arm64"
	case elf.EM_ARM:
		return "arm"
	case elf.EM_MIPS:
		arch := "mips"
		if is64 {
			arch = "mips64"
		}
		if le {
			arch += "le"
		}
		return arch
	case elf.EM_RISCV:
		return "riscv64"
	case elf.EM_LOONGARCH:
		return "loong64"
	case elf.EM_PPC64:
		if le {
			return "ppc64le"
		}
		return "ppc64"
	case elf.EM_S390:
		return "s390x"
	}
	return strings.ToLower(strings.TrimPrefix(f.Machine.String(), "EM_"))
}

func peArch(m uint16) string {
	switch m {
	case pe.IMAGE_FILE_MACHINE_AMD64:
		return "amd64"
	case pe.IMAGE_FILE_MACHINE_I386:
		return "386"
	case pe.IMAGE_FILE_MACHINE_ARM64:
		return "arm64"
	case pe.IMAGE_FILE_MACHINE_ARMNT, pe.IMAGE_FILE_MACHINE_ARM:
		return "arm"
	}
	return fmt.Sprintf("0x%x", m)
}

func machoArch(c macho.Cpu) string {
	switch c {
	case macho.CpuAmd64:
		return "amd64"
	case macho.CpuArm64:
		return "arm64"
	case macho.Cpu386:
		return "386"
	case macho.CpuArm:
		return "arm"
	}
	return strings.ToLower(c.String())
}

// compatArch 为可能在本机运行的非同名架构（是否真能运行由执行测试决定）。
var compatArch = map[string][]string{
	"amd64": {"386"},
	"arm64": {"arm", "amd64"}, // 32 位 ARM 兼容层 / macOS Rosetta
}

// CheckPlatform 判断文件平台是否可能在本机运行；ARM 不区分 GOARM 版本。
func CheckPlatform(goos, goarch string) error {
	return checkPlatform(goos, goarch, runtime.GOOS, runtime.GOARCH)
}

func checkPlatform(goos, goarch, hostOS, hostArch string) error {
	if goos == hostOS && (goarch == hostArch || slices.Contains(compatArch[hostArch], goarch)) {
		return nil
	}
	return badFile("文件为 %s/%s，本机为 %s/%s", goos, goarch, hostOS, hostArch)
}

// ---------- 版本识别 ----------

var versionRe = regexp.MustCompile(`[vV](\d+\.\d+\.\d+)`)

// ParseVersion 从文本中取出第一个 vX.Y.Z，失败返回空串。
func ParseVersion(s string) string {
	if m := versionRe.FindStringSubmatch(s); m != nil {
		return "v" + m[1]
	}
	return ""
}

// normalizeVersion 规范化用户填写的版本号（补 v 前缀、去除空白），非法时返回空串。
func normalizeVersion(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 32 || strings.ContainsAny(s, " \t\r\n/\\") {
		return ""
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "v" + s
	}
	return s
}

// probeTimeout 为试运行超时（测试中缩短）。
var probeTimeout = 5 * time.Second

// probe 在临时目录中执行 `<bin> -v` 验证可执行并读取版本号。
// cfst 输出版本后会联网检查更新，超时后结束进程即可，版本行已输出。
func probe(ctx context.Context, bin, workDir string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-v")
	cmd.Dir = workDir
	cmd.WaitDelay = time.Second
	var out limitedBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		return "", badFile("无法执行该文件: %v", err)
	}
	werr := cmd.Wait()
	v := ParseVersion(out.String())
	// 输出版本后因联网检查更新超时被结束仍视为成功；未超时却异常退出且没有版本号视为无法运行
	if werr != nil && ctx.Err() == nil && v == "" {
		return "", badFile("试运行失败: %v%s", werr, outputHead(out.String()))
	}
	return v, nil
}

// outputHead 取输出的前几行用于错误提示。
func outputHead(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 3 {
		lines = lines[:3]
	}
	if h := strings.TrimSpace(strings.Join(lines, " / ")); h != "" {
		return "（输出: " + h + "）"
	}
	return ""
}

// limitedBuffer 只保留前 4KB 输出（并发安全）。
type limitedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := 4096 - b.buf.Len(); room > 0 {
		b.buf.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// verify 校验平台并试运行二进制，返回平台与探测到的版本号。
func (m *Manager) verify(ctx context.Context, bin []byte) (ImportResult, error) {
	goos, goarch, err := DetectPlatform(bytes.NewReader(bin))
	if err != nil {
		return ImportResult{}, err
	}
	if err := CheckPlatform(goos, goarch); err != nil {
		return ImportResult{}, err
	}
	dir, err := m.tempDir()
	if err != nil {
		return ImportResult{}, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, binName())
	if err := os.WriteFile(path, bin, 0o755); err != nil {
		return ImportResult{}, err
	}
	v, err := probe(ctx, path, dir)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Version: v, OS: goos, Arch: goarch}, nil
}

// tempDir 在数据目录下创建临时目录（系统 /tmp 可能挂载为 noexec）。
func (m *Manager) tempDir() (string, error) {
	parent := filepath.Dir(m.Dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(parent, ".cfst-probe-*")
}

// ---------- 上传导入 ----------

// Import 导入上传的压缩包（zip / tar.gz）或裸可执行文件。
// 版本识别顺序：参数 version → 文件名中的 vX.Y.Z → 执行 -v 的输出 → unknown。
func (m *Manager) Import(ctx context.Context, name string, data []byte, version string) (ImportResult, error) {
	done, err := m.begin()
	if err != nil {
		return ImportResult{}, err
	}
	defer done()

	var files map[string][]byte
	switch {
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		files, err = extract(".zip", data)
	case bytes.HasPrefix(data, []byte{0x1f, 0x8b}):
		files, err = extract(".tar.gz", data)
	default:
		files = map[string][]byte{"bin": data}
	}
	if err != nil {
		return ImportResult{}, badFile("解压失败: %v", err)
	}
	return m.importFiles(ctx, files, version, name)
}

// importFiles 校验二进制后写入安装目录。
func (m *Manager) importFiles(ctx context.Context, files map[string][]byte, version, name string) (ImportResult, error) {
	res, err := m.verify(ctx, files["bin"])
	if err != nil {
		return res, err
	}
	probed := res.Version
	res.Version = firstNonEmpty(normalizeVersion(version), ParseVersion(filepath.Base(name)), probed, "unknown")
	if err := m.installFiles(files, res.Version); err != nil {
		return res, err
	}
	if err := m.ensureIPFiles(); err != nil {
		return res, err
	}
	m.Log.Info("已导入 cfst", "version", res.Version, "os", res.OS, "arch", res.Arch, "from", name)
	return res, nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// 与 cfst 发布包中 ip.txt / ipv6.txt 一致的 Cloudflare 官方 IP 段，
// 用于导入裸二进制且安装目录中尚无 IP 段文件的情况。
const (
	defaultIPv4 = "173.245.48.0/20\n103.21.244.0/22\n103.22.200.0/22\n103.31.4.0/22\n141.101.64.0/18\n108.162.192.0/18\n" +
		"190.93.240.0/20\n188.114.96.0/20\n197.234.240.0/22\n198.41.128.0/17\n162.158.0.0/15\n104.16.0.0/13\n104.24.0.0/14\n" +
		"172.64.0.0/13\n131.0.72.0/22\n"
	defaultIPv6 = "2400:cb00::/32\n2606:4700::/32\n2803:f800::/32\n2405:b500::/32\n2405:8100::/32\n2a06:98c0::/29\n2c0f:f248::/32\n"
)

// ensureIPFiles 在缺少 IP 段文件时写入内置默认值。
func (m *Manager) ensureIPFiles() error {
	for kind, content := range map[string]string{"v4": defaultIPv4, "v6": defaultIPv6} {
		if _, err := os.Stat(m.IPFile(kind)); err == nil {
			continue
		}
		if err := writeAtomic(m.IPFile(kind), []byte(content), 0o644); err != nil {
			return err
		}
		if _, err := os.Stat(m.defaultIPFile(kind)); err != nil {
			if err := writeAtomic(m.defaultIPFile(kind), []byte(content), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---------- 自动识别 ----------

var binNames = []string{"cfst", "CloudflareST"}

type found struct{ path, source string }

// candidates 列出候选文件路径（不含已登记的安装本身），不做探测。
func (m *Manager) candidates() []found {
	var out []found
	seen := map[string]bool{}
	add := func(p, source string) {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		if real, err := filepath.EvalSymlinks(p); err == nil {
			p = real
		}
		st, err := os.Stat(p)
		if err != nil || st.IsDir() || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, found{p, source})
	}
	exe := func(n string) string {
		if runtime.GOOS == "windows" {
			return n + ".exe"
		}
		return n
	}
	registered := m.Installed() && m.Version() != ""
	if registered {
		// 登记的安装本身不作为候选
		if p, err := filepath.EvalSymlinks(m.BinPath()); err == nil {
			seen[p] = true
		}
	}
	// 数据目录：安装目录内未写 VERSION 的二进制，以及直接放在数据目录根下的二进制
	for _, dir := range []string{m.Dir, filepath.Dir(m.Dir)} {
		for _, n := range binNames {
			add(filepath.Join(dir, exe(n)), "datadir")
		}
	}
	for _, n := range binNames {
		if p, err := exec.LookPath(n); err == nil {
			add(p, "path")
		}
	}
	if m.BundleDir != "" {
		add(filepath.Join(m.BundleDir, "cfst"), "bundled")
	}
	return out
}

// Scan 识别本机已有的 cfst，逐个检查平台并试运行读取版本。
// 与已登记二进制内容相同的文件（如 bundled 镜像的预置文件）不再列出。
func (m *Manager) Scan(ctx context.Context) []Candidate {
	list := m.candidates()
	var cur [32]byte
	hasCur := false
	if m.Installed() && m.Version() != "" {
		cur, hasCur = fileHash(m.BinPath())
	}
	out := make([]Candidate, len(list))
	skip := make([]bool, len(list))
	var wg sync.WaitGroup
	for i, f := range list {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if hasCur {
				if h, ok := fileHash(f.path); ok && h == cur {
					skip[i] = true
					return
				}
			}
			out[i] = m.inspect(ctx, f)
		}()
	}
	wg.Wait()
	res := []Candidate{}
	for i := range out {
		if !skip[i] {
			res = append(res, out[i])
		}
	}
	return res
}

func fileHash(p string) ([32]byte, bool) {
	f, err := os.Open(p)
	if err != nil {
		return [32]byte{}, false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, MaxUploadSize)); err != nil {
		return [32]byte{}, false
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, true
}

// inspect 检查单个候选文件。
func (m *Manager) inspect(ctx context.Context, f found) Candidate {
	c := Candidate{Path: f.path, Source: f.source}
	file, err := os.Open(f.path)
	if err != nil {
		c.Message = err.Error()
		return c
	}
	c.OS, c.Arch, err = DetectPlatform(file)
	file.Close()
	if err == nil {
		err = CheckPlatform(c.OS, c.Arch)
	}
	if err != nil {
		c.Message = err.Error()
		return c
	}
	c.Version = firstNonEmpty(bundledVersion(f), ParseVersion(filepath.Base(f.path)))
	dir, err := m.tempDir()
	if err != nil {
		c.Message = err.Error()
		return c
	}
	defer os.RemoveAll(dir)
	v, err := probe(ctx, f.path, dir)
	if err != nil {
		c.Message = err.Error()
		return c
	}
	c.Compatible = true
	c.Version = firstNonEmpty(c.Version, v)
	return c
}

// bundledVersion 读取预置目录中的 VERSION 文件。
func bundledVersion(f found) string {
	if f.source != "bundled" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(f.path), "VERSION"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// Adopt 把识别到的 cfst 复制到安装目录并写入 VERSION；path 必须是 Scan 能发现的候选之一。
func (m *Manager) Adopt(ctx context.Context, path, version string) (ImportResult, error) {
	var f *found
	for _, c := range m.candidates() {
		if p, err := filepath.EvalSymlinks(path); err == nil && p == c.path || path == c.path {
			f = &c
			break
		}
	}
	if f == nil {
		return ImportResult{}, badFile("不是可导入的 cfst 路径: %s", path)
	}
	st, err := os.Stat(f.path)
	if err != nil {
		return ImportResult{}, err
	}
	if st.Size() > MaxUploadSize {
		return ImportResult{}, badFile("文件超过 64MB")
	}

	done, err := m.begin()
	if err != nil {
		return ImportResult{}, err
	}
	defer done()
	bin, err := os.ReadFile(f.path)
	if err != nil {
		return ImportResult{}, err
	}
	files := map[string][]byte{"bin": bin}
	// 预置目录与数据目录中的同级 IP 段文件一并导入
	if f.source != "path" {
		for _, n := range []string{"ip.txt", "ipv6.txt"} {
			if b, err := os.ReadFile(filepath.Join(filepath.Dir(f.path), n)); err == nil {
				files[n] = b
			}
		}
	}
	return m.importFiles(ctx, files, firstNonEmpty(normalizeVersion(version), bundledVersion(*f)), f.path)
}
