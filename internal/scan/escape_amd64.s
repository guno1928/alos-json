//go:build amd64 && !purego

#include "textflag.h"

#include "strclass_amd64.h"

#define CLASSIFY(off, base) \
	VMOVDQU  off(base), Y0; \
	VPCMPEQB Y1, Y0, Y9; \
	VPCMPEQB Y2, Y0, Y10; \
	VPAND    Y5, Y0, Y11; \
	VPSHUFB  Y11, Y6, Y11; \
	VPSRLW   $4, Y0, Y12; \
	VPAND    Y5, Y12, Y12; \
	VPSHUFB  Y12, Y7, Y12; \
	VPAND    Y11, Y12, Y11; \
	VPOR     Y11, Y9, Y9; \
	VPAND    Y3, Y11, Y12; \
	VPCMPEQB Y4, Y12, Y12; \
	VPSLLW   $3, Y11, Y13; \
	VPAND    Y8, Y11, Y14; \
	VPCMPEQB Y4, Y14, Y14

#define MERGE_HIGH(vec, dst) \
	VPMOVMSKB vec, DI; \
	SHLQ      $32, DI; \
	ORQ       DI, dst

#define BLOCK_MASKS(base) \
	CLASSIFY(0, base); \
	VPMOVMSKB Y9, R8; \
	VPMOVMSKB Y10, R9; \
	VPMOVMSKB Y12, R10; \
	VPMOVMSKB Y13, BX; \
	VPMOVMSKB Y14, DX; \
	CLASSIFY(32, base); \
	MERGE_HIGH(Y9, R8); \
	MERGE_HIGH(Y10, R9); \
	MERGE_HIGH(Y12, R10); \
	MERGE_HIGH(Y13, BX); \
	MERGE_HIGH(Y14, DX)

TEXT ·escapedStringEndAVX2(SB), NOSPLIT, $0-24
	MOVQ    p+0(FP), SI
	MOVQ    n+8(FP), CX
	ADDQ    SI, CX
	VMOVDQU strQuote<>(SB), Y1
	VMOVDQU strBslash<>(SB), Y2
	VMOVDQU strEscBits<>(SB), Y3
	VPXOR   Y4, Y4, Y4
	VMOVDQU strNibble<>(SB), Y5
	VMOVDQU strClassLo<>(SB), Y6
	VMOVDQU strClassHi<>(SB), Y7
	VMOVDQU strHexBits<>(SB), Y8
	MOVQ    $0x5555555555555555, R13
	MOVQ    $0xAAAAAAAAAAAAAAAA, AX
	XORQ    R11, R11
	XORQ    R12, R12

loop:
	LEAQ 64(SI), DI
	CMPQ DI, CX
	JA   tail
	BLOCK_MASKS(SI)

process:
	ANDNQ R9, R11, R9
	LEAQ  (R11)(R9*2), DI
	ANDNQ R9, DI, R11
	ANDQ  AX, R11
	ADDQ  R11, R9
	SBBQ  R11, R11
	NEGQ  R11
	ADDQ  R9, R9
	XORQ  R13, R9
	ANDQ  DI, R9
	ANDNQ R8, R9, R8
	ANDQ  R9, R10
	ANDQ  R9, BX
	LEAQ  (BX)(BX*2), DI
	MOVQ  DI, R9
	SHLQ  $2, R9
	ORQ   DI, R9
	ADDQ  R9, R9
	ORQ   R12, R9
	ANDQ  DX, R9
	ORQ   R9, R10
	SHRQ  $58, BX
	ANDL  $0x3C, BX
	MOVQ  $0xFFFFFFFF77773310, R12
	SHRXQ BX, R12, R12
	ANDL  $15, R12
	TESTQ R8, R8
	JNZ   found
	TESTQ R10, R10
	JNZ   fail
	ADDQ  $64, SI
	JMP   loop

tail:
	CMPQ SI, CX
	JAE  fail
	LEAQ -64(CX), DI
	BLOCK_MASKS(DI)
	MOVQ  SI, DI
	SUBQ  CX, DI
	ADDQ  $64, DI
	SHRXQ DI, R8, R8
	SHRXQ DI, R9, R9
	SHRXQ DI, R10, R10
	SHRXQ DI, BX, BX
	SHRXQ DI, DX, DX
	JMP   process

found:
	BLSMSKQ R8, DI
	TESTQ   DI, R10
	JNZ     fail
	TZCNTQ  R8, R8
	CMPB    (SI)(R8*1), $0x22
	JNE     fail
	ADDQ    SI, R8
	SUBQ    p+0(FP), R8
	VZEROUPPER
	MOVQ    R8, ret+16(FP)
	RET

fail:
	VZEROUPPER
	MOVQ $-1, ret+16(FP)
	RET
