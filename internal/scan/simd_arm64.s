//go:build arm64 && alosneon && !purego

#include "textflag.h"

// ARM64 NEON constants - 16-byte vectors
DATA jsonSpaceNeon<>+0(SB)/8, $0x2020202020202020
DATA jsonSpaceNeon<>+8(SB)/8, $0x2020202020202020
GLOBL jsonSpaceNeon<>(SB), (NOPTR+RODATA), $16

DATA jsonQuoteNeon<>+0(SB)/8, $0x2222222222222222
DATA jsonQuoteNeon<>+8(SB)/8, $0x2222222222222222
GLOBL jsonQuoteNeon<>(SB), (NOPTR+RODATA), $16

DATA jsonBslashNeon<>+0(SB)/8, $0x5C5C5C5C5C5C5C5C
DATA jsonBslashNeon<>+8(SB)/8, $0x5C5C5C5C5C5C5C5C
GLOBL jsonBslashNeon<>(SB), (NOPTR+RODATA), $16

DATA jsonLtNeon<>+0(SB)/8, $0x3C3C3C3C3C3C3C3C
DATA jsonLtNeon<>+8(SB)/8, $0x3C3C3C3C3C3C3C3C
GLOBL jsonLtNeon<>(SB), (NOPTR+RODATA), $16

DATA jsonGtNeon<>+0(SB)/8, $0x3E3E3E3E3E3E3E3E
DATA jsonGtNeon<>+8(SB)/8, $0x3E3E3E3E3E3E3E3E
GLOBL jsonGtNeon<>(SB), (NOPTR+RODATA), $16

DATA jsonAmpNeon<>+0(SB)/8, $0x2626262626262626
DATA jsonAmpNeon<>+8(SB)/8, $0x2626262626262626
GLOBL jsonAmpNeon<>(SB), (NOPTR+RODATA), $16







// Bit mask for extracting per-byte flags
// Each byte position has bit value [1,2,4,8, 16,32,64,128, 1,2,4,8, 16,32,64,128]
DATA bitMaskNeon<>+0(SB)/8, $0x8040201008040201
DATA bitMaskNeon<>+8(SB)/8, $0x8040201008040201
GLOBL bitMaskNeon<>(SB), (NOPTR+RODATA), $16

// func findSafeStringSIMD(p *byte, n int) int
// ARM64 NEON: scans 16 bytes at a time for bytes < 0x20, '"', '\'
TEXT ·findSafeStringSIMD(SB), NOSPLIT|NOFRAME, $0-24
	MOVD p+0(FP), R0
	MOVD n+8(FP), R1
	MOVD ZR, R2            // offset = 0

	// Load constants into vector registers
	MOVD $jsonSpaceNeon<>(SB), R3
	VLD1 (R3), [V0.B16]   // V0 = 0x20 repeated
	MOVD $jsonQuoteNeon<>(SB), R3
	VLD1 (R3), [V1.B16]   // V1 = 0x22 repeated
	MOVD $jsonBslashNeon<>(SB), R3
	VLD1 (R3), [V2.B16]   // V2 = 0x5C repeated
	MOVD $bitMaskNeon<>(SB), R3
	VLD1 (R3), [V7.B16]   // V7 = bit position mask

neon_loop16:
	SUB  R2, R1, R3
	CMP  $16, R3
	BLT  neon_tail

	// Load 16 bytes
	VLD1.P 16(R0), [V3.B16]

	// bytes < 0x20: VCMHI V3, V0 => true where V0[i] > V3[i] (unsigned)
	VCMHI V3.B16, V0.B16, V4.B16

	// bytes == '"'
	VCMEQ V1.B16, V3.B16, V5.B16

	// bytes == '\'
	VCMEQ V2.B16, V3.B16, V6.B16

	// Combine
	VORR V5.B16, V4.B16, V4.B16
	VORR V6.B16, V4.B16, V4.B16

	// Extract bitmask: AND with bit pattern, then reduce
	VAND V7.B16, V4.B16, V4.B16

	// Pairwise add to compress to 2 bytes
	VADDP V4.B8, V4.B8, V5.B8    // low 8 => 4 bytes
	// Need to handle high 8 separately
	VDUP V4.D[1], V6.D2
	VADDP V6.B8, V6.B8, V6.B8    // high 8 => 4 bytes

	// Continue reducing low half
	VADDP V5.B8, V5.B8, V5.B8    // 4 => 2 bytes
	VADDP V5.B8, V5.B8, V5.B8    // 2 => 1 byte
	VMOV V5.B[0], R4              // low byte = bits 0..7

	// Continue reducing high half
	VADDP V6.B8, V6.B8, V6.B8
	VADDP V6.B8, V6.B8, V6.B8
	VMOV V6.B[0], R5              // high byte = bits 8..15

	ORR R5<<8, R4, R4            // 16-bit mask

	CBZ R4, neon_nomatch

	// Find first set bit
	RBIT R4, R5
	CLZ  R5, R5
	ADD  R5, R2, R2
	B    neon_done

neon_nomatch:
	ADD $16, R2
	B   neon_loop16

neon_tail:
	CMP R2, R1
	BLE neon_done_check

neon_done:
	MOVD R2, ret+16(FP)
	RET

neon_done_check:
	BGE neon_done_final
	MOVBU (R0), R3
	CMP $0x20, R3
	BLO neon_done_final
	CMP $0x22, R3
	BEQ neon_done_final
	CMP $0x5C, R3
	BEQ neon_done_final
	ADD $1, R0
	ADD $1, R2
	B   neon_done_check

neon_done_final:
	MOVD R2, ret+16(FP)
	RET

// func findSafeStringHTMLSIMD(p *byte, n int) int
TEXT ·findSafeStringHTMLSIMD(SB), NOSPLIT|NOFRAME, $0-24
	MOVD p+0(FP), R0
	MOVD n+8(FP), R1
	MOVD ZR, R2

	MOVD $jsonSpaceNeon<>(SB), R3
	VLD1 (R3), [V0.B16]
	MOVD $jsonQuoteNeon<>(SB), R3
	VLD1 (R3), [V1.B16]
	MOVD $jsonBslashNeon<>(SB), R3
	VLD1 (R3), [V2.B16]
	MOVD $jsonLtNeon<>(SB), R3
	VLD1 (R3), [V16.B16]
	MOVD $jsonGtNeon<>(SB), R3
	VLD1 (R3), [V17.B16]
	MOVD $jsonAmpNeon<>(SB), R3
	VLD1 (R3), [V18.B16]
	MOVD $bitMaskNeon<>(SB), R3
	VLD1 (R3), [V7.B16]

html_neon_loop:
	SUB  R2, R1, R3
	CMP  $16, R3
	BLT  html_neon_tail

	VLD1.P 16(R0), [V3.B16]

	VCMHI V3.B16, V0.B16, V4.B16
	VCMEQ V1.B16, V3.B16, V5.B16
	VCMEQ V2.B16, V3.B16, V6.B16
	VCMEQ V16.B16, V3.B16, V19.B16
	VCMEQ V17.B16, V3.B16, V20.B16
	VCMEQ V18.B16, V3.B16, V21.B16

	VORR V5.B16, V4.B16, V4.B16
	VORR V6.B16, V4.B16, V4.B16
	VORR V19.B16, V4.B16, V4.B16
	VORR V20.B16, V4.B16, V4.B16
	VORR V21.B16, V4.B16, V4.B16

	VAND V7.B16, V4.B16, V4.B16
	VADDP V4.B8, V4.B8, V5.B8
	VDUP V4.D[1], V6.D2
	VADDP V6.B8, V6.B8, V6.B8
	VADDP V5.B8, V5.B8, V5.B8
	VADDP V5.B8, V5.B8, V5.B8
	VMOV V5.B[0], R4
	VADDP V6.B8, V6.B8, V6.B8
	VADDP V6.B8, V6.B8, V6.B8
	VMOV V6.B[0], R5
	ORR R5<<8, R4, R4

	CBZ R4, html_neon_nomatch
	RBIT R4, R5
	CLZ  R5, R5
	ADD  R5, R2, R2
	B    html_neon_done

html_neon_nomatch:
	ADD $16, R2
	B   html_neon_loop

html_neon_tail:
	CMP R2, R1
	BGE html_neon_done
	MOVBU (R0), R3
	CMP $0x20, R3
	BLO html_neon_done
	CMP $0x22, R3
	BEQ html_neon_done
	CMP $0x5C, R3
	BEQ html_neon_done
	CMP $0x3C, R3
	BEQ html_neon_done
	CMP $0x3E, R3
	BEQ html_neon_done
	CMP $0x26, R3
	BEQ html_neon_done
	ADD $1, R0
	ADD $1, R2
	B   html_neon_tail

html_neon_done:
	MOVD R2, ret+16(FP)
	RET

// func skipWhitespaceSIMD(p *byte, n int) int
TEXT ·countBackslashSIMD(SB), NOSPLIT|NOFRAME, $0-24
	MOVD p+0(FP), R0
	MOVD n+8(FP), R1
	MOVD ZR, R2

	MOVD $jsonBslashNeon<>(SB), R3
	VLD1 (R3), [V0.B16]
	MOVD $bitMaskNeon<>(SB), R3
	VLD1 (R3), [V7.B16]

bs_neon_loop:
	SUB  R2, R1, R3
	CMP  $16, R3
	BLT  bs_neon_tail

	VLD1.P 16(R0), [V1.B16]
	VCMEQ V0.B16, V1.B16, V2.B16

	VAND V7.B16, V2.B16, V2.B16
	VADDP V2.B8, V2.B8, V3.B8
	VDUP V2.D[1], V4.D2
	VADDP V4.B8, V4.B8, V4.B8
	VADDP V3.B8, V3.B8, V3.B8
	VADDP V3.B8, V3.B8, V3.B8
	VMOV V3.B[0], R4
	VADDP V4.B8, V4.B8, V4.B8
	VADDP V4.B8, V4.B8, V4.B8
	VMOV V4.B[0], R5
	ORR R5<<8, R4, R4

	CBZ R4, bs_neon_nomatch
	RBIT R4, R5
	CLZ  R5, R5
	ADD  R5, R2, R2
	B    bs_neon_done

bs_neon_nomatch:
	ADD $16, R2
	B   bs_neon_loop

bs_neon_tail:
	CMP R2, R1
	BGE bs_neon_done
	MOVBU (R0), R3
	CMP $0x5C, R3
	BEQ bs_neon_done
	ADD $1, R0
	ADD $1, R2
	B   bs_neon_tail

bs_neon_done:
	MOVD R2, ret+16(FP)
	RET
