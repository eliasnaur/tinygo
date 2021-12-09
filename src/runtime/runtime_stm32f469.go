// +build stm32,stm32f469

package runtime

import (
	"device/stm32"
	"machine"
)

/*
   clock settings
   +-------------+--------+
   | HSE         | 8mhz   |
   | SYSCLK      | 180mhz |
   | HCLK        | 180mhz |
   | APB2(PCLK2) | 90mhz  |
   | APB1(PCLK1) | 45mhz  |
   +-------------+--------+
*/
const (
	HSE_STARTUP_TIMEOUT = 0x0500
	// PLL Options - See RM0386 Reference Manual pg. 148
	PLL_M = 8 // PLL_VCO = (HSE_VALUE or HSI_VALUE / PLL_M) * PLL_N
	PLL_N = 360
	PLL_P = 2 // SYSCLK = PLL_VCO / PLL_P
	PLL_Q = 7 // USB OTS FS, SDIO and RNG Clock = PLL_VCO / PLL_Q
	PLL_R = 6 // DSI
)

type arrtype = uint32

func init() {
	initCLK()

	machine.Serial.Configure(machine.UARTConfig{})

	initTickTimer(&machine.TIM2)
}

func putchar(c byte) {
	machine.Serial.WriteByte(c)
}

func initCLK() {
	// Reset clock registers
	// Set HSION
	stm32.RCC.CR.SetBits(stm32.RCC_CR_HSION)
	for !stm32.RCC.CR.HasBits(stm32.RCC_CR_HSIRDY) {
	}

	// Reset CFGR
	stm32.RCC.CFGR.Set(0x00000000)
	// Reset HSEON, CSSON and PLLON
	stm32.RCC.CR.ClearBits(stm32.RCC_CR_HSEON | stm32.RCC_CR_CSSON | stm32.RCC_CR_PLLON)
	// Reset PLLCFGR
	stm32.RCC.PLLCFGR.Set(0x24003010)
	// Reset HSEBYP
	stm32.RCC.CR.ClearBits(stm32.RCC_CR_HSEBYP)
	// Disable all interrupts
	stm32.RCC.CIR.Set(0x00000000)

	// Set up the clock
	var startupCounter uint32 = 0

	// Enable HSE
	stm32.RCC.CR.Set(stm32.RCC_CR_HSEON)

	// Wait till HSE is ready and if timeout is reached exit
	for {
		startupCounter++
		if stm32.RCC.CR.HasBits(stm32.RCC_CR_HSERDY) || (startupCounter == HSE_STARTUP_TIMEOUT) {
			break
		}
	}
	if stm32.RCC.CR.HasBits(stm32.RCC_CR_HSERDY) {
		// Enable high performance mode, System frequency up to 180MHz
		stm32.RCC.APB1ENR.SetBits(stm32.RCC_APB1ENR_PWREN)
		const PWR_CR_VOS = stm32.PWR_CR_VOS_Msk
		stm32.PWR.CR.SetBits(PWR_CR_VOS)
		// HCLK = SYSCLK / 1
		stm32.RCC.CFGR.SetBits(stm32.RCC_CFGR_HPRE_Div1 << stm32.RCC_CFGR_HPRE_Pos)
		// PCLK2 = HCLK / 2
		stm32.RCC.CFGR.SetBits(stm32.RCC_CFGR_PPRE2_Div2 << stm32.RCC_CFGR_PPRE2_Pos)
		// PCLK1 = HCLK / 4
		stm32.RCC.CFGR.SetBits(stm32.RCC_CFGR_PPRE1_Div4 << stm32.RCC_CFGR_PPRE1_Pos)
		// Configure the main PLL
		// PLL Options - See RM0386 Reference Manual pg. 148
		stm32.RCC.PLLCFGR.Set(PLL_M | (PLL_N << stm32.RCC_PLLCFGR_PLLN_Pos) | (((PLL_P >> 1) - 1) << stm32.RCC_PLLCFGR_PLLP_Pos) |
			(1 << stm32.RCC_PLLCFGR_PLLSRC_Pos) | (PLL_Q << stm32.RCC_PLLCFGR_PLLQ_Pos) | (PLL_R << stm32.RCC_PLLCFGR_PLLR_Pos))
		// Enable main PLL
		stm32.RCC.CR.SetBits(stm32.RCC_CR_PLLON)
		// Wait till the main PLL is ready
		for (stm32.RCC.CR.Get() & stm32.RCC_CR_PLLRDY) == 0 {
		}
		// Configure Flash prefetch, Instruction cache, Data cache and wait state
		stm32.FLASH.ACR.Set(stm32.FLASH_ACR_ICEN | stm32.FLASH_ACR_DCEN | (5 << stm32.FLASH_ACR_LATENCY_Pos))
		// Select the main PLL as system clock source
		stm32.RCC.CFGR.ClearBits(stm32.RCC_CFGR_SW_Msk)
		stm32.RCC.CFGR.SetBits(stm32.RCC_CFGR_SW_PLL << stm32.RCC_CFGR_SW_Pos)
		for (stm32.RCC.CFGR.Get() & stm32.RCC_CFGR_SWS_Msk) != (stm32.RCC_CFGR_SWS_PLL << stm32.RCC_CFGR_SWS_Pos) {
		}

	} else {
		// If HSE failed to start up, the application will have wrong clock configuration
		for {
		}
	}

	// Enable the CCM RAM clock
	stm32.RCC.AHB1ENR.SetBits(stm32.RCC_AHB1ENR_CCMDATARAMEN)
}
