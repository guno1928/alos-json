//go:build amd64 && !purego

#include "textflag.h"

// Constants for string escape scanning
DATA jsonEscSpace<>+0(SB)/8, $0x1F1F1F1F1F1F1F1F
DATA jsonEscSpace<>+8(SB)/8, $0x1F1F1F1F1F1F1F1F
GLOBL jsonEscSpace<>(SB), (NOPTR+RODATA), $16

DATA jsonEscQuote<>+0(SB)/8, $0x2222222222222222
DATA jsonEscQuote<>+8(SB)/8, $0x2222222222222222
GLOBL jsonEscQuote<>(SB), (NOPTR+RODATA), $16

DATA jsonEscBslash<>+0(SB)/8, $0x5C5C5C5C5C5C5C5C
DATA jsonEscBslash<>+8(SB)/8, $0x5C5C5C5C5C5C5C5C
GLOBL jsonEscBslash<>(SB), (NOPTR+RODATA), $16

// HTML extra chars: <(0x3C), >(0x3E), &(0x26)
DATA jsonEscLT<>+0(SB)/8, $0x3C3C3C3C3C3C3C3C
DATA jsonEscLT<>+8(SB)/8, $0x3C3C3C3C3C3C3C3C
GLOBL jsonEscLT<>(SB), (NOPTR+RODATA), $16

DATA jsonEscGT<>+0(SB)/8, $0x3E3E3E3E3E3E3E3E
DATA jsonEscGT<>+8(SB)/8, $0x3E3E3E3E3E3E3E3E
GLOBL jsonEscGT<>(SB), (NOPTR+RODATA), $16

DATA jsonEscAmp<>+0(SB)/8, $0x2626262626262626
DATA jsonEscAmp<>+8(SB)/8, $0x2626262626262626
GLOBL jsonEscAmp<>(SB), (NOPTR+RODATA), $16

// Whitespace constants






DATA bslashVec<>+0(SB)/8, $0x5C5C5C5C5C5C5C5C
DATA bslashVec<>+8(SB)/8, $0x5C5C5C5C5C5C5C5C
GLOBL bslashVec<>(SB), (NOPTR+RODATA), $16

// func findSafeStringSIMD(p *byte, n int) int
// Uses SSE2 to scan 16 bytes at a time for bytes < 0x20, '"' (0x22), '\' (0x5C)
// Returns offset of first such byte, or n if none found
TEXT ·findSafeStringSIMD(SB), NOSPLIT|NOFRAME, $0-24
	MOVQ p+0(FP), SI
	MOVQ n+8(FP), CX
	XORQ AX, AX

	MOVOU jsonEscSpace<>(SB), X0   // X0 = [0x1F x 16]
	MOVOU jsonEscQuote<>(SB), X1   // X1 = [0x22 x 16]
	MOVOU jsonEscBslash<>(SB), X2  // X2 = [0x5C x 16]
	PXOR  X5, X5                   // X5 = zeros

loop16:
	MOVQ CX, DX
	SUBQ AX, DX
	CMPQ DX, $16
	JL   tail

	MOVOU (SI)(AX*1), X3

	// --- Check bytes < 0x20 ---
	// PSUBUSB X0, X4 => X4[i] = saturating_sub(X3[i], 0x1F)
	// If X3[i] < 0x20, result is 0
	MOVOU X3, X4
	PSUBUSB X0, X4
	PCMPEQB X5, X4   // X4 = 0xFF where X3[i] < 0x20

	// --- Check bytes == '"' (0x22) ---
	MOVOU X3, X6
	PCMPEQB X1, X6   // X6 = 0xFF where X3[i] == '"'

	// --- Check bytes == '\' (0x5C) ---
	MOVOU X3, X7
	PCMPEQB X2, X7   // X7 = 0xFF where X3[i] == '\'

	// --- Combine masks ---
	POR  X6, X4
	POR  X7, X4

	// --- Extract bitmask ---
	PMOVMSKB X4, DX

	TESTL DX, DX
	JNZ   found16

	ADDQ $16, AX
	JMP  loop16

found16:
	BSFL DX, DX
	ADDQ DX, AX
	JMP  done

tail:
	CMPQ AX, CX
	JGE  done

	MOVBLZX (SI)(AX*1), DX
	CMPB DL, $0x20
	JB   done
	CMPB DL, $0x22
	JE   done
	CMPB DL, $0x5C
	JE   done

	INCQ AX
	JMP  tail

done:
	MOVQ AX, ret+16(FP)
	RET

// func findSafeStringHTMLSIMD(p *byte, n int) int
// Same as above but also detects '<' (0x3C), '>' (0x3E), '&' (0x26)
TEXT ·findSafeStringHTMLSIMD(SB), NOSPLIT|NOFRAME, $0-24
	MOVQ p+0(FP), SI
	MOVQ n+8(FP), CX
	XORQ AX, AX

	MOVOU jsonEscSpace<>(SB), X0
	MOVOU jsonEscQuote<>(SB), X1
	MOVOU jsonEscBslash<>(SB), X2
	MOVOU jsonEscLT<>(SB), X8
	MOVOU jsonEscGT<>(SB), X9
	MOVOU jsonEscAmp<>(SB), X10
	PXOR  X5, X5

html_loop16:
	MOVQ CX, DX
	SUBQ AX, DX
	CMPQ DX, $16
	JL   html_tail

	MOVOU (SI)(AX*1), X3

	// bytes < 0x20
	MOVOU X3, X4
	PSUBUSB X0, X4
	PCMPEQB X5, X4

	// == '"'
	MOVOU X3, X6
	PCMPEQB X1, X6

	// == '\'
	MOVOU X3, X7
	PCMPEQB X2, X7

	// == '<'
	MOVOU X3, X11
	PCMPEQB X8, X11

	// == '>'
	MOVOU X3, X12
	PCMPEQB X9, X12

	// == '&'
	MOVOU X3, X13
	PCMPEQB X10, X13

	// Combine all
	POR  X6, X4
	POR  X7, X4
	POR  X11, X4
	POR  X12, X4
	POR  X13, X4

	PMOVMSKB X4, DX
	TESTL DX, DX
	JNZ   html_found16

	ADDQ $16, AX
	JMP  html_loop16

html_found16:
	BSFL DX, DX
	ADDQ DX, AX
	JMP  html_done

html_tail:
	CMPQ AX, CX
	JGE  html_done

	MOVBLZX (SI)(AX*1), DX
	CMPB DL, $0x20
	JB   html_done
	CMPB DL, $0x22
	JE   html_done
	CMPB DL, $0x5C
	JE   html_done
	CMPB DL, $0x3C
	JE   html_done
	CMPB DL, $0x3E
	JE   html_done
	CMPB DL, $0x26
	JE   html_done

	INCQ AX
	JMP  html_tail

html_done:
	MOVQ AX, ret+16(FP)
	RET

// func skipWhitespaceSIMD(p *byte, n int) int
// Scans for non-whitespace characters (whitespace = space, tab, \n, \r, comma, colon)
// Returns offset of first non-whitespace byte
TEXT ·countBackslashSIMD(SB), NOSPLIT|NOFRAME, $0-24
	MOVQ p+0(FP), SI
	MOVQ n+8(FP), CX
	XORQ AX, AX

	MOVOU bslashVec<>(SB), X0

bs_loop16:
	MOVQ CX, DX
	SUBQ AX, DX
	CMPQ DX, $16
	JL   bs_tail

	MOVOU (SI)(AX*1), X1
	PCMPEQB X0, X1
	PMOVMSKB X1, DX

	TESTL DX, DX
	JNZ   bs_found

	ADDQ $16, AX
	JMP  bs_loop16

bs_found:
	BSFL DX, DX
	ADDQ DX, AX
	JMP  bs_done

bs_tail:
	CMPQ AX, CX
	JGE  bs_done
	CMPB (SI)(AX*1), $0x5C
	JE   bs_done
	INCQ AX
	JMP  bs_tail

bs_done:
	MOVQ AX, ret+16(FP)
	RET

// func quoteCopySIMD(src *byte, srcLen int, dst *byte, dstCap int) int
// Fused scan+copy: copies bytes from src to dst while scanning for JSON special chars.
// Stops at the first byte that needs escaping (< 0x20, '"', '\').
// Returns the number of bytes copied (== number of safe bytes consumed).
// This fuses the scan and copy into a single SIMD pass, halving memory traffic.
TEXT ·quoteCopySIMD(SB), NOSPLIT|NOFRAME, $0-40
	MOVQ src+0(FP), SI      // source pointer
	MOVQ srcLen+8(FP), CX   // source length
	MOVQ dst+16(FP), DI     // dest pointer
	MOVQ dstCap+24(FP), R8  // dest capacity
	XORQ AX, AX             // offset = 0

	// Use the smaller of srcLen and dstCap as our limit
	CMPQ CX, R8
	JLE  qc_use_src
	MOVQ R8, CX
qc_use_src:

	MOVOU jsonEscSpace<>(SB), X0   // X0 = [0x1F x 16]
	MOVOU jsonEscQuote<>(SB), X1   // X1 = [0x22 x 16]
	MOVOU jsonEscBslash<>(SB), X2  // X2 = [0x5C x 16]
	PXOR  X5, X5                   // X5 = zeros

qc_loop16:
	MOVQ CX, DX
	SUBQ AX, DX
	CMPQ DX, $16
	JL   qc_tail

	// Load 16 bytes from source
	MOVOU (SI)(AX*1), X3

	// Check for special bytes: < 0x20, == '"', == '\'
	MOVOU X3, X4
	PSUBUSB X0, X4
	PCMPEQB X5, X4           // X4 = 0xFF where src[i] < 0x20

	MOVOU X3, X6
	PCMPEQB X1, X6           // X6 = 0xFF where src[i] == '"'

	MOVOU X3, X7
	PCMPEQB X2, X7           // X7 = 0xFF where src[i] == '\'

	POR  X6, X4
	POR  X7, X4

	PMOVMSKB X4, DX
	TESTL DX, DX
	JNZ   qc_found16

	// All 16 bytes are safe — copy them to dst
	MOVOU X3, (DI)(AX*1)
	ADDQ $16, AX
	JMP  qc_loop16

qc_found16:
	// Find first special byte position
	BSFL DX, DX
	// Copy the safe bytes before the special one
	// DX = number of safe bytes in this chunk
	TESTL DX, DX
	JZ    qc_done_add

	// Copy DX bytes (0..15), small memcpy
	// Use overlapping 8-byte copies for efficiency
	CMPQ DX, $8
	JL   qc_copy_small

	// Copy first 8 bytes
	MOVQ (SI)(AX*1), R9
	MOVQ R9, (DI)(AX*1)
	// Copy last 8 bytes (may overlap): addr = base + offset + dx - 8
	LEAQ (AX)(DX*1), R10
	LEAQ -8(R10), R10
	MOVQ (SI)(R10*1), R9
	MOVQ R9, (DI)(R10*1)
	JMP  qc_done_add

qc_copy_small:
	// DX is 1..7
	CMPQ DX, $4
	JL   qc_copy_tiny
	// Copy first 4 bytes
	MOVL (SI)(AX*1), R9
	MOVL R9, (DI)(AX*1)
	// Copy last 4 bytes (may overlap)
	LEAQ (AX)(DX*1), R10
	LEAQ -4(R10), R10
	MOVL (SI)(R10*1), R9
	MOVL R9, (DI)(R10*1)
	JMP  qc_done_add

qc_copy_tiny:
	// DX is 1..3
	CMPQ DX, $1
	JE   qc_copy_1
	CMPQ DX, $2
	JE   qc_copy_2
	// DX == 3
	MOVW (SI)(AX*1), R9
	MOVW R9, (DI)(AX*1)
	MOVB 2(SI)(AX*1), R9
	MOVB R9, 2(DI)(AX*1)
	JMP  qc_done_add
qc_copy_2:
	MOVW (SI)(AX*1), R9
	MOVW R9, (DI)(AX*1)
	JMP  qc_done_add
qc_copy_1:
	MOVB (SI)(AX*1), R9
	MOVB R9, (DI)(AX*1)

qc_done_add:
	ADDQ DX, AX
	JMP  qc_done

qc_tail:
	CMPQ AX, CX
	JGE  qc_done

	MOVBLZX (SI)(AX*1), DX
	CMPB DL, $0x20
	JB   qc_done
	CMPB DL, $0x22
	JE   qc_done
	CMPB DL, $0x5C
	JE   qc_done

	// Safe byte — copy it
	MOVB DL, (DI)(AX*1)
	INCQ AX
	JMP  qc_tail

qc_done:
	MOVQ AX, ret+32(FP)
	RET
