package runtime

import "unsafe"
import "runtime/internal/atomic"

const NOP = 0xe320f000

//for booting
func Runtime_main()

//go:nosplit
func PutR0(val uint32)

//go:nosplit
func PutSP(val uint32)

//go:nosplit
func PutR2(val uint32)

//go:nosplit
func RR0() uint32

//go:nosplit
func RR1() uint32

//go:nosplit
func RR2() uint32

//go:nosplit
func RR3() uint32

//go:nosplit
func RR4() uint32

//go:nosplit
func RR5() uint32

//go:nosplit
func RR6() uint32

//go:nosplit
func RR7() uint32

//go:nosplit
func RSP() uint32

////catching traps
var firstexit = true

var writelock Spinlock_t

//go:nosplit
func trap_debug() {
	arg0 := RR0()
	arg1 := RR1()
	arg2 := RR2()
	arg3 := RR3()
	arg4 := RR4()
	arg5 := RR5()
	arg6 := RR6()
	trapno := RR7()
	//print("incoming trap: ", trapno, "\n")
	switch trapno {
	case 1:
		print("exit")
		PutR0(0)
	case 3:
		ret := uint32(0xffffffff)
		PutR0(ret)
		return
	case 4:
		ret := uint32(0xffffffff)
		if arg0 == 1 || arg0 == 2 {
			splock(&writelock)
			ret = write_uart_unsafe(uintptr(arg1), arg2)
			spunlock(&writelock)
		} else {
		}
		PutR0(ret)
		return
	case 5:
		PutR0(0xffffffff)
		return
	case 6:
		PutR0(0)
		return
	case 120:
		thread_id := makethread(uint32(arg0), uintptr(arg1), uintptr(arg2))
		PutR0(uint32(thread_id))
		return
	case 142:
		if !panicpanic {
			splock(threadlock)
			curthread[cpunum()].state = ST_RUNNABLE
			return_here()
		}
		PutR0(0)
		return
	case 158:
		splock(threadlock)
		curthread[cpunum()].state = ST_RUNNABLE
		return_here()
		PutR0(0)
		return
	case 174:
		PutR0(0)
		return
	case 175:
		PutR0(0)
		return
	case 186:
		PutR0(0)
		return
	case 192:
		throw("mmap trap\n")
		addr := unsafe.Pointer(uintptr(arg0))
		n := uintptr(arg1)
		prot := int32(arg2)
		flags := int32(arg3)
		fd := int32(arg4)
		off := uint32(arg5)
		ret := uint32(uintptr(hack_mmap(addr, n, prot, flags, fd, off)))
		PutR0(ret)
		return
	case 220:
		PutR0(0)
		return
	case 224:
		PutR0(thread_current())
		return
	case 238:
		PutR0(0)
		return
	case 240:
		uaddr := ((*int32)(unsafe.Pointer(uintptr(arg0))))
		ts := ((*timespec)(unsafe.Pointer(uintptr(arg3))))
		uaddr2 := ((*int32)(unsafe.Pointer(uintptr(arg4))))
		ret := hack_futex_arm(uaddr, int32(arg1), int32(arg2), ts, uaddr2, int32(arg5))
		PutR0(uint32(ret))
		return
	case 248:
		if firstexit == true {
			firstexit = false
			PutR0(0)
		}
		for {
		}
	case 263:
		clock_type := arg0
		ts := ((*timespec)(unsafe.Pointer(uintptr(arg1))))
		clk_gettime(clock_type, ts)
		PutR0(0)
		return
	case 0xbadbabe:
		throw("abort")
	}
	print("unpatched trap: ", trapno, " on cpu ", cpunum(), "\n")
	print("\tr0: ", hex(arg0), "\n")
	print("\tr1: ", hex(arg1), "\n")
	print("\tr2: ", hex(arg2), "\n")
	print("\tr3: ", hex(arg3), "\n")
	print("\tr4: ", hex(arg4), "\n")
	print("\tr5: ", hex(arg5), "\n")
	print("\tr6: ", hex(arg6), "\n")
	throw("trap")
}

//
// THREADING
//

//go:nosplit
func Threadschedule()

//go:nosplit
func RecordTrapframe()

//go:nosplit
func ReplayTrapframe(cputhread *thread_t)

const maxcpus = 4

//cpu states
const (
	CPU_WFI      = 0
	CPU_STARTED  = 1
	CPU_RELEASED = 2
	CPU_FULL     = 3
)

var cpustatus [maxcpus]uint32

type trapframe struct {
	lr  uintptr
	sp  uintptr
	fp  uintptr
	r0  uint32
	r1  uint32
	r2  uint32
	r3  uint32
	r10 uint32
}

type thread_t struct {
	tf       trapframe
	state    uint32
	futaddr  uintptr
	sleeptil timespec
	id       uint32
	lock     Spinlock_t
}

// maximum # of runtime "OS" threads
const maxthreads = 64

var threads [maxthreads]thread_t
var curthread [4]*thread_t

// thread states
const (
	ST_INVALID   = 0
	ST_RUNNABLE  = 1
	ST_RUNNING   = 2
	ST_WAITING   = 3
	ST_SLEEPING  = 4
	ST_WILLSLEEP = 5
	ST_NEW       = 6
)

//go:nosplit
func thread_init() {
	for i := uint32(0); i < maxthreads; i++ {
		va := uintptr(unsafe.Pointer(&threads[i].id))
		pgnum := va >> PGSHIFT
		pde := (*uint32)(unsafe.Pointer(kernpgdir + uintptr(pgnum*4)))
		if (*pde & 0x2) == 0 {
			print("UNMAPPED thread structure in thread_init")
		}
		threads[i].id = i
	}
	threads[0].state = ST_RUNNING
	mycpu := cpunum()
	curthread[mycpu] = &threads[0]
	RecordTrapframe()
}

//go:nosplit
func makethread(flags uint32, stack uintptr, entry uintptr) int {
	CLONE_VM := 0x100
	CLONE_FS := 0x200
	CLONE_FILES := 0x400
	CLONE_SIGHAND := 0x800
	CLONE_THREAD := 0x10000
	chk := uint32(CLONE_VM | CLONE_FS | CLONE_FILES | CLONE_SIGHAND |
		CLONE_THREAD)
	if flags != chk {
		throw("clone error")
	}
	i := 0
	for ; i < maxthreads; i++ {
		if threads[i].state == ST_INVALID {
			break
		}
	}
	if i == maxthreads {
		throw("out of threads to use\n")
	}
	threads[i].state = ST_RUNNABLE
	threads[i].tf.lr = entry
	threads[i].tf.sp = stack
	threads[i].tf.r0 = 0 //returns 0 in the child
	//print("\t\t\t\t new thread ", i, "\n")
	return int(i)
}

var lastrun = 0

var threadlock *Spinlock_t

var thread_time timespec

var threadstacks [4]uint32

//go:nosplit
func thread_schedule() {
	for {
		mycpu := cpunum()
		//check if any futex timed out
		clk_gettime(0, &thread_time)
		for next := (lastrun + 1) % maxthreads; next != lastrun; next = (next + 1) % maxthreads {

			//wake sleepers
			if threads[next].state == ST_SLEEPING {
				if (thread_time.tv_sec >= threads[next].sleeptil.tv_sec) || (thread_time.tv_sec == threads[next].sleeptil.tv_sec && thread_time.tv_nsec >= threads[next].sleeptil.tv_nsec) {
					threads[next].state = ST_RUNNABLE
					threads[next].futaddr = 0
					threads[next].tf.r0 = 0
					threads[next].sleeptil.tv_sec = 0
					threads[next].sleeptil.tv_nsec = 0
				}
			}

			//we found something runnable
			if threads[next].state == ST_RUNNABLE {
				threads[next].state = ST_RUNNING
				curthread[mycpu] = &threads[next]
				cpustatus[mycpu] = CPU_FULL
				lastrun = next
				invallpages()
				spunlock(threadlock)
				if uintptr(unsafe.Pointer(curthread[mycpu])) == 0 || curthread[mycpu] == nil {
					panic("trapframe is null")
				}
				ReplayTrapframe(curthread[mycpu])
				throw("should never be here\n")
			}
		}

		//check lastrun if we can just go back to that
		if threads[lastrun].state == ST_RUNNABLE {
			threads[lastrun].state = ST_RUNNING
			curthread[mycpu] = &threads[lastrun]
			cpustatus[mycpu] = CPU_FULL
			lastrun = lastrun
			invallpages()
			spunlock(threadlock)
			ReplayTrapframe(curthread[mycpu])
			throw("should never be here\n")
		}

		//drop to idle, there's nothing to do
		spunlock(threadlock)
		idle()
		splock(threadlock)
	}
	throw("no runnable threads. what happened?\n")
}

//go:nosplit
func idle() {
}

//go:nosplit
func thread_current() uint32 {
	return curthread[cpunum()].id
	//return uint32(lastrun)
}

//go:nosplit
func bkpt() {
	write_uart([]byte(">"))
	return
}

//go:nosplit
func return_here() {
	Threadschedule()
}

var temptime timespec

//go:nosplit
func hack_futex_arm(uaddr *int32, op, val int32, to *timespec, uaddr2 *int32, val2 int32) int32 {
	FUTEX_WAIT := int32(0)
	FUTEX_WAKE := int32(1)
	uaddrn := uintptr(unsafe.Pointer(uaddr))
	ret := 0
	mycpu := cpunum()
	switch op {
	case FUTEX_WAIT:
		dosleep := *uaddr == val
		if dosleep {
			//enter thread scheduler
			splock(threadlock)
			curthread[mycpu].state = ST_SLEEPING
			curthread[mycpu].futaddr = uaddrn
			curthread[mycpu].sleeptil.tv_sec = 0
			curthread[mycpu].sleeptil.tv_nsec = 0
			if to != nil {
				clk_gettime(0, &temptime)
				curthread[mycpu].sleeptil.tv_sec = temptime.tv_sec + to.tv_sec
				curthread[mycpu].sleeptil.tv_nsec = temptime.tv_nsec + to.tv_nsec
			}
			return_here()
			ret = int(RR0())
		} else {
			//lost wakeup?
			eagain := -11
			ret = eagain
		}
	case FUTEX_WAKE:
		woke := 0
		for i := 0; i < maxthreads && val > 0; i++ {
			t := &threads[i]
			st := t.state
			if t.futaddr == uaddrn && st == ST_SLEEPING {
				t.state = ST_RUNNABLE
				t.futaddr = 0
				t.tf.r0 = 0
				val--
				woke++
			}
		}
		ret = woke
	default:
		throw("unexpected futex op")
	}
	return int32(ret)
}

//
// VIRTUAL MEMORY
//

type physaddr uint32

//go:nosplit
func loadttbr0(l1base unsafe.Pointer)

//go:nosplit
func loadvbar(vbar_addr unsafe.Pointer)

//go:nosplit
func Readvbar() uint32

//go:nosplit
func invallpages()

//go:nosplit
func DMB()

//This file will have all the things to do with the arm MMU and page tables
//assume we will be addressing 4gb of memory
//using the short descriptor page format

const RAM_START = physaddr(0x10000000)
const RAM_SIZE = uint32(0x80000000)
const ONE_MEG = uint32(0x00100000)

const PERIPH_START = physaddr(0x110000)
const PERIPH_SIZE = uint32(0x0FFFFFFF)

//1MB pages
const PGSIZE = uint32(0x100000)
const PGSHIFT = uint32(20)
const L1_ALIGNMENT = uint32(0x4000)
const VBAR_ALIGNMENT = uint32(0x20)

var kernelstart physaddr
var kernelsize physaddr
var bootstack physaddr

type Interval struct {
	start uint32
	size  uint32
}

const ELF_MAGIC = 0x464C457F
const ELF_PROG_LOAD = 1

type Elf struct {
	magic       uint32
	e_elf       [12]uint8
	e_type      uint16
	e_machine   uint16
	e_version   uint32
	e_entry     uint32
	e_phoff     uint32
	e_shoff     uint32
	e_flags     uint32
	e_ehsize    uint16
	e_phentsize uint16
	e_phnum     uint16
	e_shentsize uint16
	e_shnum     uint16
	e_shstrndx  uint16
}

type Proghdr struct {
	p_type   uint32
	p_offset uint32
	p_va     uint32
	p_pa     uint32
	p_filesz uint32
	p_memsz  uint32
	p_flags  uint32
	p_align  uint32
}

const KERNEL_END = physaddr(0x40200000)

var boot_end physaddr

const PageInfoSz = uint32(8)

type PageInfo struct {
	next_pageinfo uintptr
	ref           uint32
}

//linear array of struct PageInfos
var npages uint32
var pages uintptr
var pgnfosize uint32 = uint32(8)

//pointer to the next PageInfo to give away
var nextfree uintptr

//L1 table
var l1_table physaddr

//each cpu gets an interrupt stack
//and a boot stack
var isr_stack [4]physaddr
var isr_stack_pt *physaddr = &isr_stack[0]

//vector table
//8 things
//reset, undefined, svc, prefetch abort, data abort, unused, irq, fiq
type vector_table struct {
	reset           uint32
	undefined       uint32
	svc             uint32
	prefetch_abort  uint32
	data_abort      uint32
	_               uint32
	irq             uint32
	fiq             uint32
	reset_addr      uint32
	undef_addr      uint32
	svc_addr        uint32
	prefetch_addr   uint32
	abort_addr      uint32
	_               uint32
	irq_addr        uint32
	fiq_addr        uint32
	reset_addr_addr uint32
}

var vectab *vector_table

//linear array of page directory entries that form the kernel pgdir
var kernpgdir uintptr

//go:nosplit
func roundup(val, upto uint32) uint32 {
	result := (val + (upto - 1)) & ^(upto - 1)
	return result
}

//go:nosplit
func verifyzero(addr uintptr, n uint32) {
	for i := 0; i < int(n); i++ {
		test := *((*byte)(unsafe.Pointer(addr + uintptr(n))))
		if test > 0 {
		}
	}
}

//go:nosplit
func memclrbytes(ptr unsafe.Pointer, n uintptr)

//go:nosplit
func boot_alloc(size uint32) physaddr {
	result := boot_end
	newsize := uint32(roundup(uint32(size), 0x4))
	boot_end = boot_end + physaddr(newsize)
	memclrNoHeapPointers(unsafe.Pointer(uintptr(result)), uintptr(newsize))
	DMB()
	return result
}

/*
* Turn on the 64bit ARM global timer.
*It will never roll over unless gert runs for over 1000 years.
 */
//go:nosplit
func clock_init() {
	global_timer.control = 0
	global_timer.counter_lo = 0
	global_timer.counter_hi = 0
	global_timer.control = 1
}

//go:nosplit
func mem_init() {
	npages = RAM_SIZE / PGSIZE

	//allocate the l1 table
	//4 bytes each and 4096 entries
	boot_end = physaddr(roundup(uint32(bootstack), L1_ALIGNMENT))
	l1_table = boot_alloc(4 * 4096)

	//allocate the vector table
	boot_end = physaddr(roundup(uint32(boot_end), VBAR_ALIGNMENT))
	vectab = ((*vector_table)(unsafe.Pointer(uintptr(boot_alloc(uint32(unsafe.Sizeof(vector_table{})))))))
	vbar := Readvbar()
	boot_table := ((*vector_table)(unsafe.Pointer(uintptr(vbar))))
	*vectab = *boot_table
	catch := getcatch()
	abort := getabort_int()
	pref_abort := getpref_abort()
	undefined := getundefined()
	//replace push lr at the start of catch
	*((*uint32)(unsafe.Pointer(uintptr(catch)))) = NOP
	vectab.reset_addr = catch
	vectab.undef_addr = undefined
	vectab.svc_addr = catch
	vectab.prefetch_addr = pref_abort
	vectab.abort_addr = abort
	vectab.irq_addr = catch
	vectab.fiq_addr = catch
	vectab.reset_addr_addr = catch

	//allocate the threadlock
	threadlock = (*Spinlock_t)(unsafe.Pointer(uintptr(boot_alloc(uint32(4)))))

	//allocate the maplock
	maplock = (*Spinlock_t)(unsafe.Pointer(uintptr(boot_alloc(uint32(4)))))

	//allocate the timelock
	timelock = (*Spinlock_t)(unsafe.Pointer(uintptr(boot_alloc(uint32(4)))))

	//allocate scheduler stacks
	boot_alloc(4 * 1028)
	end := uint32(boot_alloc(0))
	for i := uint32(0); i < 4; i++ {
		threadstacks[i] = (end - 1024*i) & uint32(0xFFFFFFF8)
	}

	pages = uintptr(boot_alloc(npages * 8))
	physPageSize = uintptr(PGSIZE)

}

//go:nosplit
func pgnum2pa(pgnum uint32) physaddr {
	return physaddr(PGSIZE * pgnum)
}

//go:nosplit
func pa2page(pa physaddr) *PageInfo {
	pgnum := uint32(uint32(pa) / PGSIZE)
	return (*PageInfo)(unsafe.Pointer((uintptr(unsafe.Pointer(uintptr(pages))) + uintptr(pgnum*pgnfosize))))
	//return uintptr(pages) + uintptr(pgnum*pgnfosize)
}

//go:nosplit
func pa2pgnum(pa physaddr) uint32 {
	return uint32(pa) / PGSIZE
}

//go:nosplit
func pageinfo2pa(pgnfo *PageInfo) physaddr {
	pgnum := uint32((uintptr(unsafe.Pointer(pgnfo)) - pages) / unsafe.Sizeof(PageInfo{}))
	return pgnum2pa(pgnum)
}

//go:nosplit
func check_page_free(pgnfo *PageInfo) bool {
	curpage := (*PageInfo)(unsafe.Pointer(nextfree))
	for {
		if pgnfo == curpage {
			return true
		}
		if curpage.next_pageinfo == 0 {
			break
		}
		curpage = (*PageInfo)(unsafe.Pointer(curpage.next_pageinfo))
	}
	return false
}

//go:nosplit
func walk_pgdir(pgdir uintptr, va uint32) *uint32 {
	table_index := va >> PGSHIFT
	pte := (*uint32)(unsafe.Pointer(pgdir + uintptr(4*table_index)))
	return pte
}

//go:nosplit
func zeropage(pa physaddr) {
	memclrNoHeapPointers(unsafe.Pointer(uintptr(pa)), uintptr(PGSIZE))
}

//go:nosplit
func page_init() {
	//construct a linked-list of free pages
	//print("start page init\n")
	nfree := uint32(0)
	nextfree = 0
	for i := pa2pgnum(RAM_START); i < pa2pgnum(physaddr(uint32(RAM_START)+RAM_SIZE)); i++ {
		pa := pgnum2pa(i)
		pagenfo := pa2page(pa)
		if pa >= physaddr(RAM_START) && pa < kernelstart {
			//zeropage(pa)
			pagenfo.next_pageinfo = nextfree
			pagenfo.ref = 0
			nextfree = uintptr(unsafe.Pointer(pagenfo))
			nfree += 1
		} else if pa >= physaddr(KERNEL_END) && pa < physaddr(uint32(RAM_START)+uint32(RAM_SIZE)-uint32(ONE_MEG)) {
			//zeropage(pa)
			pagenfo.next_pageinfo = nextfree
			pagenfo.ref = 0
			nextfree = uintptr(unsafe.Pointer(pagenfo))
			nfree += 1
		} else {
			pagenfo.ref = 1
			pagenfo.next_pageinfo = 0
		}
	}
	//print("page init done\n")
	//print("free pages: ", nfree, "\n")
	npagenfo := (*PageInfo)(unsafe.Pointer(nextfree))
	print("next free page is for pa: ", hex(pageinfo2pa(npagenfo)), "\n")
}

//go:nosplit
func page_alloc() *PageInfo {
	freepage := (*PageInfo)(unsafe.Pointer(nextfree))
	nextfree = freepage.next_pageinfo
	return freepage
}

//go:nosplit
func checkcontiguousfree(pgdir uintptr, va, size uint32) bool {
	for start := va; start < va+size; start += PGSIZE {
		////print("checkcontiguous va: ", hex(start), " size: ", hex(size), "\n")
		pgnum := start >> PGSHIFT
		pde := (*uint32)(unsafe.Pointer(pgdir + uintptr(pgnum*4)))
		if *pde&0x2 > 0 {
			////print("\tfalse: ", hex(*pde), "\n")
			return false
		}
	}
	////print("found contiguous\n")
	return true
}

//device memory, TEX[2:0] = 0b010, S=0
const MEM_TYPE_DEVICE = uint32(0x2 << 12)

//cacheable write-back shareable for SMP
//TEX[2:0] = 0b001, C=1, B=1, S=1
const MEM_NORMAL_SMP = uint32(0x1<<12 | 0x1<<16 | 0x1<<3 | 0x1<<2)

//go:nosplit
func map_region(pa uint32, va uint32, size uint32, perms uint32) {
	//section entry bits
	pa = pa & 0xFFF00000
	va = va & 0xFFF00000
	perms = perms | 0x2
	realsize := roundup(size, PGSIZE)
	i := uint32(0)
	for ; i <= realsize; i += PGSIZE {
		nextpa := pa + i
		l1offset := nextpa >> 18
		entry := (*uint32)(unsafe.Pointer(uintptr(l1_table + physaddr(l1offset))))
		base_addr := (va + i)
		*entry = base_addr | perms
	}
}

//go:nosplit
func Unmap_region(pa uint32, va uint32, size uint32) {
	//section entry bits
	pa = pa & 0xFFF00000
	va = va & 0xFFF00000
	realsize := roundup(size, PGSIZE)
	i := uint32(0)
	for ; i <= realsize; i += PGSIZE {
		nextpa := pa + i
		l1offset := nextpa >> 18
		entry := (*uint32)(unsafe.Pointer(uintptr(l1_table + physaddr(l1offset))))
		*entry = 0
	}
}

//go:nosplit
func map_kernel() {
	//read the kernel elf to find the regions of the kernel
	elf := ((*Elf)(unsafe.Pointer(uintptr(kernelstart))))
	if elf.magic != ELF_MAGIC {
		throw("bad elf header in the kernel\n")
	}

	for i := uintptr(0); i < uintptr(elf.e_phnum); i++ {
		ph := ((*Proghdr)(unsafe.Pointer(uintptr(elf.e_phoff) + uintptr(i*unsafe.Sizeof(Proghdr{})) + uintptr(kernelstart))))
		if ph.p_type == ELF_PROG_LOAD {
			filesz := ph.p_filesz
			pa := ph.p_pa
			va := ph.p_va
			print("\tkernel pa: ", hex(pa), " va: ", hex(va), " size: ", hex(filesz), "\n")
			map_region(pa, va, filesz, MEM_TYPE_DEVICE)
		}
	}

	//install the kernel page table

	//map the uart
	map_region(0x02000000, 0x02000000, PGSIZE, MEM_TYPE_DEVICE)

	//map the timer
	map_region(uint32(PERIPH_START), uint32(PERIPH_START), PGSIZE, MEM_TYPE_DEVICE)

	//map the stack and boot_alloc scratch space
	nextfree := boot_alloc(0)
	if uint32(nextfree) < (uint32(RAM_START) + RAM_SIZE - ONE_MEG) {
		throw("out of scratch space\n")
	}
	map_region(uint32(uint32(RAM_START)+RAM_SIZE-ONE_MEG), uint32(RAM_START)+RAM_SIZE-ONE_MEG, PGSIZE, MEM_TYPE_DEVICE)

	cpustatus[0] = CPU_FULL
	//map the boot rom
	//unfortunately this has to happen in order to boot other cpus
	map_region(uint32(0x0), uint32(0x0), 256*PGSIZE, MEM_TYPE_DEVICE)

	loadvbar(unsafe.Pointer(vectab))
	loadttbr0(unsafe.Pointer(uintptr(l1_table)))
	kernpgdir = (uintptr)(unsafe.Pointer(uintptr(l1_table)))
}

//go:nosplit
func showl1table() {
	print("l1 table: ", hex(uint32(l1_table)), "\n")
	print("__________________________\n")
	for i := uint32(0); i < 4096; i += 1 {
		entry := *(*uint32)(unsafe.Pointer((uintptr(l1_table)) + uintptr(i*4)))
		if entry == 0 {
			continue
		}
		base := entry & 0xFFF00000
		perms := entry & 0x3
		print("\t| entry: ", i, ", base: ", hex(base), " perms: ", hex(perms), "\n")
	}
	print("__________________________\n")
}

//go:nosplit
func l1_walk() {
}

type Spinlock_t uint32

//go:nosplit
func splock(l *Spinlock_t) {
	for {
		if atomic.ReCas((*uint32)(unsafe.Pointer(l)), 0, 1) {
			return
		}
	}
}

//go:nosplit
func spunlock(l *Spinlock_t) {
	*l = 0
}

type Ticketlock_t struct {
	v      uint32
	holder int
}

var next_ticket int64 = -1
var now_serving int64 = 0

//go:nosplit
func (l *Ticketlock_t) lock() {
	me := atomic.Xaddint64(&next_ticket, 1)
	for me != now_serving {
	}
}

//go:nosplit
func (l *Ticketlock_t) unlock() {
	now_serving = now_serving + 1
	DMB()
}

const MMAP_FIXED = uint32(0x10)

var maplock *Spinlock_t

// mmap calls the mmap system call.  It is implemented in assembly.
//go:nosplit
func hack_mmap(addr unsafe.Pointer, n uintptr, prot, flags, fd int32, off uint32) unsafe.Pointer {
	splock(maplock)
	va := uintptr(addr)
	size := uint32(roundup(uint32(n), PGSIZE))
	print("mmap_arm: ", hex(va), " ", hex(n), " ", hex(prot), " ", hex(flags), "\n")

	if va == 0 {
		for pgnum := uint32(KERNEL_END >> PGSHIFT); pgnum < 4096; pgnum += 1 {
			if checkcontiguousfree(kernpgdir, uint32(pgnum<<PGSHIFT), size) == true {
				va = uintptr(pgnum << PGSHIFT)
				break
			}
		}
		if va == 0 {
			throw("cant find large enough chunk of contiguous virtual memory\n")
		}
	}

	//Question: what should we do when the runtime calls mmap on addresses that are
	//already mapped?
	// Linux says zero them and re-map
	for start := va; start < (va + uintptr(size)); start += uintptr(PGSIZE) {
		pte := walk_pgdir(kernpgdir, uint32(start))
		page := page_alloc()
		if page == nil {
			throw("mmap_arm: out of memory\n")
		}
		pa := pageinfo2pa(page) & 0xFFF00000
		*pte = uint32(pa) | 0x2 | MEM_TYPE_DEVICE
		memclrNoHeapPointers(unsafe.Pointer(start), uintptr(PGSIZE))
	}
	invallpages()
	DMB()
	spunlock(maplock)
	return unsafe.Pointer(va)
}

var timelock *Spinlock_t

//go:nosplit
func clk_gettime(clock_type uint32, ts *timespec) {
	splock(timelock)
	//ticks_per_sec := 0x9502f900
	//2**32 * (5/2) *1E9 ~=10.737 ==> 43/4
	ticks := ReadClock(globaltimerbase+0x4, globaltimerbase+0x0)
	nsec := (ticks * 5) >> 1
	sec := 0
	for ; nsec >= 1000000000; nsec -= 1000000000 {
		sec += 1
	}
	ts.tv_sec = int32(sec)
	ts.tv_nsec = int32(nsec)
	spunlock(timelock)
}

//
// MULTICORE BOOT
//

//go:nosplit
func cpunum() int

//go:nosplit
func Cpunum() int {
	return cpunum()
}

//go:nosplit
func boot_any()

//go:nosplit
func getentry() uint32

//go:nosplit
func catch()

//go:nosplit
func getcatch() uint32

//go:nosplit
func abort_int()

//go:nosplit
func getabort_int() uint32

//go:nosplit
func pref_abort()

//go:nosplit
func getpref_abort() uint32

//go:nosplit
func undefined()

//go:nosplit
func getundefined() uint32

//go:nosplit
func Mull64(a, b uint32) (uint32, uint32)

//go:nosplit
func Getloc(loc uint32) uint32

//go:nosplit
func ReadClock(hi, low uintptr) int64

const Mpcorebase uintptr = uintptr(0xA00000)

var scubase uintptr = Mpcorebase + 0x0
var globaltimerbase uintptr = Mpcorebase + 0x200

type GlobalTimer_regs struct {
	counter_lo uint32
	counter_hi uint32
	control    uint32
	intr       uint32
	compare_lo uint32
	compare_hi uint32
	auto_inc   uint32
}

var global_timer *GlobalTimer_regs = ((*GlobalTimer_regs)(unsafe.Pointer(globaltimerbase)))

var ns_per_clock_num uint32 = 5
var ns_per_clock_denom uint32 = 2
var sec_per_ns_denom uint32 = 1000000000

//1:0x20d8028 2:0x20d8030 3:0x20d8038 scr:0x20d8000

var scr *uint32 = ((*uint32)(unsafe.Pointer(uintptr(0x20d8000))))
var cpu1bootaddr *uint32 = ((*uint32)(unsafe.Pointer(uintptr(0x20d8028))))
var cpu1bootarg *uint32 = ((*uint32)(unsafe.Pointer(uintptr(0x20d802C))))

var cpu2bootaddr *uint32 = ((*uint32)(unsafe.Pointer(uintptr(0x20d8030))))
var cpu2bootarg *uint32 = ((*uint32)(unsafe.Pointer(uintptr(0x20d8034))))

var cpu3bootaddr *uint32 = ((*uint32)(unsafe.Pointer(uintptr(0x20d8038))))
var cpu3bootarg *uint32 = ((*uint32)(unsafe.Pointer(uintptr(0x20d803C))))

var bootlock Spinlock_t

//go:nosplit
func cleardcache()

//go:nosplit
func scu_enable()

//go:nosplit
func isr_setup()

//go:nosplit
func mp_init() {

	//SMP bring up is described in section 5.3.4 of DDI 0407G

	//set up stacks, they must be 8 byte aligned

	//first set up isr_stack
	//other cores will boot off the isr stack
	start := uint32(boot_alloc(4 * 1028))
	end := uint32(boot_alloc(0))

	print("start stack: ", hex(start), " end stack: ", hex(end), "\n")
	for i := uint32(0); i < 4; i++ {
		isr_stack[i] = physaddr((end - 1024*i) & 0xFFFFFFF8)
		print("cpu[", i, "] isr stack at ", hex(isr_stack[i]), "\n")
	}

	if cpunum() != 0 {
	}
	cpustatus[0] = CPU_FULL

	scu_enable()

	//set the interrupt stack
	isr_setup()

	entry := getentry()
	//replace the push lr at the start of entry with a nop
	*((*uint32)(unsafe.Pointer(uintptr(entry)))) = NOP
	//cpu1
	*cpu1bootaddr = entry
	*cpu1bootarg = uint32(isr_stack[1])
	//cpu2
	*cpu2bootaddr = entry
	*cpu2bootarg = uint32(isr_stack[2])
	//cpu3
	*cpu3bootaddr = entry
	*cpu3bootarg = uint32(isr_stack[3])

}

//go:nosplit
func mp_pen() {
	me := cpunum()
	loadvbar(unsafe.Pointer(vectab))
	loadttbr0(unsafe.Pointer(uintptr(l1_table)))
	cpustatus[me] = CPU_STARTED
	trampoline()
	throw("cpu release\n")
}

type GIC_cpu_map struct {
	cpu_interface_control_register       uint32
	interrupt_priority_mask_register     uint32
	binary_point_register                uint32
	interrupt_acknowledge_register       uint32
	end_of_interrupt_register            uint32
	running_priority_register            uint32
	highest_pending_interrupt_register   uint32
	aliased_binary_point_register        uint32
	reserved1                            [8]uint32
	implementation_defined_registers     [36]uint32
	reserved2                            [11]uint32
	cpu_interface_dentification_register uint32
}

var gic_cpu *GIC_cpu_map = (*GIC_cpu_map)(unsafe.Pointer(uintptr(Getmpcorebase() + 0x100)))

//go:nosplit
func trampoline() {
	//have to enable the GIC cpu interface here for each cpu
	EnableIRQ()
	me := cpunum()
	gic_cpu.cpu_interface_control_register = 0x03   // enable everything
	gic_cpu.interrupt_priority_mask_register = 0xFF //unmask everything
	cpustatus[me] = CPU_RELEASED
	splock(threadlock)
	thread_schedule()
}

//go:nosplit
func Release(ncpu uint) {
	//boot n cpus
	start := uint(1)
	for cpu := start; cpu < start+ncpu; cpu++ {
		print("boot cpu ", cpu, " ... ")
		for *scr&(0x1<<(13+cpu)|0x1<<(17+cpu)) > 0 {
		}
		*scr |= 0x1 << (21 + cpu)
		for cpustatus[cpu] == CPU_WFI {
		}
		print("done!\n")
	}
}

//
// INTERRUPTS
//

var trapfn func(irqnum uint32)
var Booted uint8

///The world might be stopped, we dont really know
//go:nosplit
//go:nowritebarrierrec
func cpucatch() {
	/*
	* Make sure only one cpu gets the interrupt?
	 */
	irqnum := gic_cpu.interrupt_acknowledge_register
	gic_cpu.end_of_interrupt_register = irqnum
	g := getg()
	if g == nil && Booted == 0 {
	} else {
		trapfn(irqnum)
	}
}

//
// FAULTS
//

//go:nosplit
//go:nowritebarrierrec
func cpuabort() {
	addr := RR0()
	mem_addr := RR1()
	me := cpunum()
	pte := walk_pgdir(kernpgdir, addr)
	pte_mem := walk_pgdir(kernpgdir, mem_addr)
	print("data abort on cpu ", me, " from instruction on addr ", hex(addr), ". Faulting memory addr: ", hex(mem_addr), "\n")
	print("pte for this instruction addr is : ", hex(*pte), "\n")
	print("pte for the fault addr is : ", hex(*pte_mem), "\n")
	print("zerobase is ", hex(zerobase))
	zerobase_addr := uintptr(unsafe.Pointer(&zerobase))
	print("zerobase addr is ", hex(zerobase_addr))
	print("wait for JTAG connection\n")
	WB_DEFAULT_UART.getchar()
}

//go:nosplit
//go:nowritebarrierrec
func cpuprefabort() {
	addr := RR0()
	me := cpunum()
	pte := walk_pgdir(kernpgdir, addr)
	print("prefetch abort on cpu ", me, " from addr ", hex(addr), "\n")
	print("pte for this addr is : ", hex(*pte))
	print("wait for JTAG connection\n")
	WB_DEFAULT_UART.getchar()
}

//go:nosplit
//go:nowritebarrierrec
func cpuundefined() {
	addr := RR0()
	me := cpunum()
	print("undefined on cpu ", me, " from addr ", hex(addr), "\n")
	print("wait for JTAG connection\n")
	WB_DEFAULT_UART.getchar()
}

//go:nosplit
func EnableIRQ()

//go:nosplit
func DisableIRQ()

//go:nosplit
func Getmpcorebase() uintptr

//
// UNUSED - Signal Handlers for Interrupts
//

var irqfunc uintptr = 0

//go:nosplit
func hack_sigaction(signum uint32, new *sigactiont, old *sigactiont, size uintptr) int32 {
	switch signum {
	case _SIGINT:
		print("sigaction on sigint\n")
		print("using sa_handler ", hex(new.sa_handler), "\n")
		irqfunc = new.sa_handler
	}
	return 0
}

//go:nosplit
func hack_sigaltstack(new, old *stackt) {
	if new == nil {
		//print("requesting signal stack\n")
		me := cpunum()
		old.ss_sp = ((*byte)(unsafe.Pointer(uintptr(isr_stack[me] - 1024))))
		old.ss_size = 1024
		old.ss_flags = 0
		return
	}
	if old == nil {
		//print("sigaltstack ", hex(uintptr(unsafe.Pointer(new.ss_sp))), "\n")
	}
	return
}

//go:nosplit
func disable_interrupts()

//go:nosplit
func enable_interrupts()
