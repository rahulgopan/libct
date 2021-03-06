package libct

// #cgo CFLAGS: -DCONFIG_X86_64 -DARCH="x86" -D_FILE_OFFSET_BITS=64 -D_GNU_SOURCE
// #cgo LDFLAGS: -l:libct.a -l:libnl-route-3.a -l:libnl-3.a -l:libapparmor.a -l:libselinux.a -lm
// #include "../src/include/uapi/libct.h"
// #include "../src/include/uapi/libct-errors.h"
import "C"
import "fmt"
import "os"
import "unsafe"

const (
	LIBCT_OPT_AUTO_PROC_MOUNT = C.LIBCT_OPT_AUTO_PROC_MOUNT
	CAPS_BSET                 = C.CAPS_BSET
	CAPS_ALLCAPS              = C.CAPS_ALLCAPS
	CAPS_ALL                  = C.CAPS_ALL
)

type Session struct {
	s C.libct_session_t
}

type Container struct {
	ct C.ct_handler_t
}

type NetDev struct {
	dev C.ct_net_t
}

type NetRoute struct {
	route C.ct_net_route_t
}

type NetRouteNextHop struct {
	nh C.ct_net_route_nh_t
}

type LibctError struct {
	Code int
}

func (e LibctError) Error() string {
	return fmt.Sprintf("LibctError: %x", e.Code)
}

func (s *Session) OpenLocal() error {
	h := C.libct_session_open_local()

	if C.libct_handle_is_err(unsafe.Pointer(h)) != 0 {
		return LibctError{int(C.libct_handle_to_err(unsafe.Pointer(h)))}
	}

	s.s = h

	return nil
}

func (s *Session) ContainerCreate(name string) (*Container, error) {
	ct := C.libct_container_create(s.s, C.CString(name))

	if C.libct_handle_is_err(unsafe.Pointer(ct)) != 0 {
		return nil, LibctError{int(C.libct_handle_to_err(unsafe.Pointer(ct)))}
	}

	return &Container{ct}, nil
}

func (s *Session) ContainerOpen(name string) (*Container, error) {
	ct := C.libct_container_open(s.s, C.CString(name))

	if C.libct_handle_is_err(unsafe.Pointer(ct)) != 0 {
		return nil, LibctError{int(C.libct_handle_to_err(unsafe.Pointer(ct)))}
	}

	return &Container{ct}, nil
}

func (s *Session) ProcessCreateDesc() (*ProcessDesc, error) {
	p := C.libct_process_desc_create(s.s)
	if C.libct_handle_is_err(unsafe.Pointer(p)) != 0 {
		return nil, LibctError{int(C.libct_handle_to_err(unsafe.Pointer(p)))}
	}

	return &ProcessDesc{desc: p}, nil
}

func (ct *Container) SetNsMask(nsmask uint64) error {
	ret := C.libct_container_set_nsmask(ct.ct, C.ulong(nsmask))

	if ret != 0 {
		return LibctError{int(ret)}
	}

	return nil
}

func (ct *Container) Kill() error {
	ret := C.libct_container_kill(ct.ct)

	if ret != 0 {
		return LibctError{int(ret)}
	}

	return nil
}

func (ct *Container) SetConsoleFd(f *os.File) error {
	ret := C.libct_container_set_console_fd(ct.ct, C.int(f.Fd()))

	if ret != 0 {
		return LibctError{int(ret)}
	}

	return nil
}

func (ct *Container) SpawnExecve(p *ProcessDesc, path string, argv []string, env []string) (error) {
	var (
		i   int = 0
	)

	type F func(*ProcessDesc) (*os.File, error)
	for _, setupFd := range []F{(*ProcessDesc).stdin, (*ProcessDesc).stdout, (*ProcessDesc).stderr} {
		fd, err := setupFd(p)
		if err != nil {
			p.closeDescriptors(p.closeAfterStart)
			p.closeDescriptors(p.closeAfterWait)
			return err
		}
		p.childFiles = append(p.childFiles, fd)
		i = i + 1
	}

	p.childFiles = append(p.childFiles, p.ExtraFiles...)

	err := ct.execve(p, path, argv, env, true)

	return err
}

func (ct *Container) EnterExecve(p *ProcessDesc, path string, argv []string, env []string) (error) {
	err := ct.execve(p, path, argv, env, false)
	p.closeDescriptors(p.closeAfterStart)
	return err
}

func (ct *Container) execve(p *ProcessDesc, path string, argv []string, env []string, spawn bool) (error) {
	var (
		h     C.ct_process_t
	)

	cargv := make([]*C.char, len(argv)+1)
	for i, arg := range argv {
		cargv[i] = C.CString(arg)
	}

	cenv := make([]*C.char, len(env)+1)
	for i, e := range env {
		cenv[i] = C.CString(e)
	}

	cfds := make([]C.int, len(p.childFiles))
	for i, fd := range p.childFiles {
		cfds[i] = C.int(fd.Fd())
	}

	C.libct_process_desc_set_fds(p.desc, &cfds[0], C.int(len(p.childFiles)))

	if spawn {
		h = C.libct_container_spawn_execve(ct.ct, p.desc, C.CString(path), &cargv[0], &cenv[0])
	} else {
		h = C.libct_container_enter_execve(ct.ct, p.desc, C.CString(path), &cargv[0], &cenv[0])
	}

	if C.libct_handle_is_err(unsafe.Pointer(h)) != 0 {
		return  LibctError{int(C.libct_handle_to_err(unsafe.Pointer(h)))}
	}

	p.handle = h

	return nil
}

func (ct *Container) Wait() error {
	ret := C.libct_container_wait(ct.ct)

	if ret != 0 {
		return LibctError{int(ret)}
	}

	return nil
}

func (ct *Container) Uname(host *string, domain *string) error {
	var chost *C.char
	var cdomain *C.char

	if host != nil {
		chost = C.CString(*host)
	}

	if domain != nil {
		cdomain = C.CString(*domain)
	}

	ret := C.libct_container_uname(ct.ct, chost, cdomain)

	if ret != 0 {
		return LibctError{int(ret)}
	}

	return nil
}

func (ct *Container) SetRoot(root string) error {

	if ret := C.libct_fs_set_root(ct.ct, C.CString(root)); ret != 0 {
		return LibctError{int(ret)}
	}

	return nil
}

const (
	CT_FS_RDONLY      = C.CT_FS_RDONLY
	CT_FS_PRIVATE     = C.CT_FS_PRIVATE
	CT_FS_NOEXEC      = C.CT_FS_NOEXEC
	CT_FS_NOSUID      = C.CT_FS_NOSUID
	CT_FS_NODEV       = C.CT_FS_NODEV
	CT_FS_STRICTATIME = C.CT_FS_STRICTATIME
)

func (ct *Container) AddBindMount(src string, dst string, flags int) error {

	if ret := C.libct_fs_add_bind_mount(ct.ct, C.CString(src), C.CString(dst), C.int(flags)); ret != 0 {
		return LibctError{int(ret)}
	}

	return nil
}

func (ct *Container) AddMount(src string, dst string, flags int, fstype string, data string) error {

	if ret := C.libct_fs_add_mount(ct.ct, C.CString(src), C.CString(dst), C.int(flags), C.CString(fstype), C.CString(data)); ret != 0 {
		return LibctError{int(ret)}
	}

	return nil
}

func (ct *Container) SetOption(opt int32) error {
	if ret := C.libct_container_set_option(ct.ct, C.int(opt), nil); ret != 0 {
		return LibctError{int(ret)}
	}

	return nil
}

func (ct *Container) AddDeviceNode(path string, mode int, major int, minor int) error {

	ret := C.libct_fs_add_devnode(ct.ct, C.CString(path), C.int(mode), C.int(major), C.int(minor))

	if ret != 0 {
		return LibctError{int(ret)}
	}

	return nil
}

func (nd *NetDev) GetPeer() (*NetDev, error) {

	dev := C.libct_net_dev_get_peer(nd.dev)

	if C.libct_handle_is_err(unsafe.Pointer(dev)) != 0 {
		return nil, LibctError{int(C.libct_handle_to_err(unsafe.Pointer(dev)))}
	}

	return &NetDev{dev}, nil
}

func (ct *Container) AddNetVeth(host_name string, ct_name string) (*NetDev, error) {

	var args C.struct_ct_net_veth_arg

	args.host_name = C.CString(host_name)
	args.ct_name = C.CString(ct_name)

	dev := C.libct_net_add(ct.ct, C.CT_NET_VETH, unsafe.Pointer(&args))

	if C.libct_handle_is_err(unsafe.Pointer(dev)) != 0 {
		return nil, LibctError{int(C.libct_handle_to_err(unsafe.Pointer(dev)))}
	}

	return &NetDev{dev}, nil
}

func (dev *NetDev) AddIpAddr(addr string) error {
	err := C.libct_net_dev_add_ip_addr(dev.dev, C.CString(addr))
	if err != 0 {
		return LibctError{int(err)}
	}

	return nil
}

func (dev *NetDev) SetMaster(master string) error {
	err := C.libct_net_dev_set_master(dev.dev, C.CString(master))
	if err != 0 {
		return LibctError{int(err)}
	}

	return nil
}

func (dev *NetDev) SetMtu(mtu int) error {
	err := C.libct_net_dev_set_mtu(dev.dev, C.int(mtu))
	if err != 0 {
		return LibctError{int(err)}
	}

	return nil
}

func (ct *Container) AddRoute() (*NetRoute, error) {
	r := C.libct_net_route_add(ct.ct)

	if C.libct_handle_is_err(unsafe.Pointer(r)) != 0 {
		return nil, LibctError{int(C.libct_handle_to_err(unsafe.Pointer(r)))}
	}

	return &NetRoute{r}, nil
}

func (route *NetRoute) SetSrc(src string) error {
	err := C.libct_net_route_set_src(route.route, C.CString(src))
	if err != 0 {
		return LibctError{int(err)}
	}

	return nil
}

func (route *NetRoute) SetDst(dst string) error {
	err := C.libct_net_route_set_dst(route.route, C.CString(dst))
	if err != 0 {
		return LibctError{int(err)}
	}

	return nil
}

func (route *NetRoute) SetDev(dev string) error {
	err := C.libct_net_route_set_dev(route.route, C.CString(dev))
	if err != 0 {
		return LibctError{int(err)}
	}

	return nil
}

func (route *NetRoute) AddNextHop() (*NetRouteNextHop, error) {
	nh := C.libct_net_route_add_nh(route.route)
	if C.libct_handle_is_err(unsafe.Pointer(nh)) != 0 {
		return nil, LibctError{int(C.libct_handle_to_err(unsafe.Pointer(nh)))}
	}

	return &NetRouteNextHop{nh}, nil
}

func (nh *NetRouteNextHop) SetGateway(addr string) error {
	err := C.libct_net_route_nh_set_gw(nh.nh, C.CString(addr))
	if err != 0 {
		return LibctError{int(err)}
	}

	return nil
}

func (nh *NetRouteNextHop) SetDev(dev string) error {
	err := C.libct_net_route_nh_set_dev(nh.nh, C.CString(dev))
	if err != 0 {
		return LibctError{int(err)}
	}

	return nil
}
