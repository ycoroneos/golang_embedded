# The Go Programming Language Modified to run on baremetal ARMv7a

Now you can write high-level, concurrent embedded programs without
the weight of Linux slowing it down.

This repo tracks the master branch of the main go repo at https://github.com/golang/go.

This modified runtime is also an integral part of G.E.R.T,
the Golang Embedded RunTime

##Usage
The freescale iMX6Quad is hard-coded in. If you are using this SOC
then you are in luck! In your Go program include a stub like this:
```
//go:nosplit
func Entry() {
	runtime.Armhackmode = 1
	runtime.Runtime_main()
}
```

and then compile your Go program by specifying the entry function and
its link address like so in a Makefile:
```
GOLINKFLAGS := "-T <link_address> -E main.Entry"
```

To set the callback function for IRQs and release other cpus from the
holding pen, do this somewhere in your program:
```
//set IRQ callback function
	runtime.SetIRQcallback(irq)

	//Release spinning cpus
	runtime.Release()
```

You will need a bootloader to boot your shiny new baremetal Go program.
I recommend uBoot.

##Modification
If the iMX6Quad is not your SOC then you can still use this code but you
will have to write some drivers.

Write a new UART driver in src/runtime/write_err.go

Modify map_kernel() in src/runtime/gert_arm.go to reflect your memory map

Modify mp_init() in src/runtime/gert_arm.go as well as the associated
assembly routines in src/runtime/gertasm_arm.s to boot your other cpus

I will give a definitive list of things to modify once I actually finish
this project
