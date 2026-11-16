//go:build amd64 && !purego

#include "textflag.h"

#define SHUFFLE_BY_LEN 0
#define POW10_TABLE 272
#define TEN16 400
#define LEN_GUESS 432
#define DIV1E8_MAGIC 496

DATA itoaDiv10k<>+0(SB)/8, $0x00000000d1b71759
DATA itoaDiv10k<>+8(SB)/8, $0x00000000d1b71759
GLOBL itoaDiv10k<>(SB), (NOPTR+RODATA), $16

DATA itoa10k<>+0(SB)/8, $0x0000000000002710
DATA itoa10k<>+8(SB)/8, $0x0000000000002710
GLOBL itoa10k<>(SB), (NOPTR+RODATA), $16

DATA itoaMul5243<>+0(SB)/8, $0x147b147b147b147b
DATA itoaMul5243<>+8(SB)/8, $0x147b147b147b147b
GLOBL itoaMul5243<>(SB), (NOPTR+RODATA), $16

DATA itoaMul100<>+0(SB)/8, $0x0064006400640064
DATA itoaMul100<>+8(SB)/8, $0x0064006400640064
GLOBL itoaMul100<>(SB), (NOPTR+RODATA), $16

DATA itoaMul6554<>+0(SB)/8, $0x199a199a199a199a
DATA itoaMul6554<>+8(SB)/8, $0x199a199a199a199a
GLOBL itoaMul6554<>(SB), (NOPTR+RODATA), $16

DATA itoaMul2559<>+0(SB)/8, $0x09ff09ff09ff09ff
DATA itoaMul2559<>+8(SB)/8, $0x09ff09ff09ff09ff
GLOBL itoaMul2559<>(SB), (NOPTR+RODATA), $16

DATA itoaZeros<>+0(SB)/8, $0x3030303030303030
DATA itoaZeros<>+8(SB)/8, $0x3030303030303030
GLOBL itoaZeros<>(SB), (NOPTR+RODATA), $16

DATA itoaTables<>+0(SB)/8, $0xffffffffffffffff
DATA itoaTables<>+8(SB)/8, $0xffffffffffffffff
DATA itoaTables<>+16(SB)/8, $0xffffffffffffff0f
DATA itoaTables<>+24(SB)/8, $0xffffffffffffffff
DATA itoaTables<>+32(SB)/8, $0xffffffffffff0f0e
DATA itoaTables<>+40(SB)/8, $0xffffffffffffffff
DATA itoaTables<>+48(SB)/8, $0xffffffffff0f0e0d
DATA itoaTables<>+56(SB)/8, $0xffffffffffffffff
DATA itoaTables<>+64(SB)/8, $0xffffffff0f0e0d0c
DATA itoaTables<>+72(SB)/8, $0xffffffffffffffff
DATA itoaTables<>+80(SB)/8, $0xffffff0f0e0d0c0b
DATA itoaTables<>+88(SB)/8, $0xffffffffffffffff
DATA itoaTables<>+96(SB)/8, $0xffff0f0e0d0c0b0a
DATA itoaTables<>+104(SB)/8, $0xffffffffffffffff
DATA itoaTables<>+112(SB)/8, $0xff0f0e0d0c0b0a09
DATA itoaTables<>+120(SB)/8, $0xffffffffffffffff
DATA itoaTables<>+128(SB)/8, $0x0f0e0d0c0b0a0908
DATA itoaTables<>+136(SB)/8, $0xffffffffffffffff
DATA itoaTables<>+144(SB)/8, $0x0e0d0c0b0a090807
DATA itoaTables<>+152(SB)/8, $0xffffffffffffff0f
DATA itoaTables<>+160(SB)/8, $0x0d0c0b0a09080706
DATA itoaTables<>+168(SB)/8, $0xffffffffffff0f0e
DATA itoaTables<>+176(SB)/8, $0x0c0b0a0908070605
DATA itoaTables<>+184(SB)/8, $0xffffffffff0f0e0d
DATA itoaTables<>+192(SB)/8, $0x0b0a090807060504
DATA itoaTables<>+200(SB)/8, $0xffffffff0f0e0d0c
DATA itoaTables<>+208(SB)/8, $0x0a09080706050403
DATA itoaTables<>+216(SB)/8, $0xffffff0f0e0d0c0b
DATA itoaTables<>+224(SB)/8, $0x0908070605040302
DATA itoaTables<>+232(SB)/8, $0xffff0f0e0d0c0b0a
DATA itoaTables<>+240(SB)/8, $0x0807060504030201
DATA itoaTables<>+248(SB)/8, $0xff0f0e0d0c0b0a09
DATA itoaTables<>+256(SB)/8, $0x0706050403020100
DATA itoaTables<>+264(SB)/8, $0x0f0e0d0c0b0a0908
DATA itoaTables<>+272(SB)/8, $0x0000000000000001
DATA itoaTables<>+280(SB)/8, $0x000000000000000a
DATA itoaTables<>+288(SB)/8, $0x0000000000000064
DATA itoaTables<>+296(SB)/8, $0x00000000000003e8
DATA itoaTables<>+304(SB)/8, $0x0000000000002710
DATA itoaTables<>+312(SB)/8, $0x00000000000186a0
DATA itoaTables<>+320(SB)/8, $0x00000000000f4240
DATA itoaTables<>+328(SB)/8, $0x0000000000989680
DATA itoaTables<>+336(SB)/8, $0x0000000005f5e100
DATA itoaTables<>+344(SB)/8, $0x000000003b9aca00
DATA itoaTables<>+352(SB)/8, $0x00000002540be400
DATA itoaTables<>+360(SB)/8, $0x000000174876e800
DATA itoaTables<>+368(SB)/8, $0x000000e8d4a51000
DATA itoaTables<>+376(SB)/8, $0x000009184e72a000
DATA itoaTables<>+384(SB)/8, $0x00005af3107a4000
DATA itoaTables<>+392(SB)/8, $0x00038d7ea4c68000
DATA itoaTables<>+400(SB)/8, $0x002386f26fc10000
DATA itoaTables<>+408(SB)/8, $0x016345785d8a0000
DATA itoaTables<>+416(SB)/8, $0x0de0b6b3a7640000
DATA itoaTables<>+424(SB)/8, $0x8ac7230489e80000
DATA itoaTables<>+432(SB)/8, $0x0302020201010101
DATA itoaTables<>+440(SB)/8, $0x0505040404040303
DATA itoaTables<>+448(SB)/8, $0x0707070706060605
DATA itoaTables<>+456(SB)/8, $0x0a0a090909080808
DATA itoaTables<>+464(SB)/8, $0x0c0c0c0b0b0b0a0a
DATA itoaTables<>+472(SB)/8, $0x0f0e0e0e0d0d0d0d
DATA itoaTables<>+480(SB)/8, $0x1111101010100f0f
DATA itoaTables<>+488(SB)/8, $0x1313131312121211
DATA itoaTables<>+496(SB)/8, $0xabcc77118461cefd
GLOBL itoaTables<>(SB), (NOPTR+RODATA), $504

#define ABS(v, sign) \
	MOVQ v, sign; \
	SARQ $63, sign; \
	XORQ sign, v; \
	SUBQ sign, v

#define DECIMAL_LEN(v, n) \
	MOVQ v, n; \
	ORQ $1, n; \
	BSRQ n, n; \
	MOVBQZX LEN_GUESS(R12)(n*1), n; \
	CMPQ v, POW10_TABLE(R12)(n*8); \
	SBBQ $-1, n

#define SPLIT_PACK(v) \
	MOVQ v, AX; \
	MULQ DIV1E8_MAGIC(R12); \
	SHRQ $26, DX; \
	IMUL3Q $100000000, DX, AX; \
	SUBQ AX, v; \
	SHLQ $32, v; \
	ORQ DX, v

#define WIDE_SPLIT \
	MOVQ R8, AX; \
	XORL DX, DX; \
	DIVQ TEN16(R12); \
	MOVQ AX, R8; \
	MOVQ DX, R9

#define XMM_CONSTANTS \
	MOVOU itoaDiv10k<>(SB), X8; \
	MOVOU itoa10k<>(SB), X9; \
	MOVOU itoaMul5243<>(SB), X10; \
	MOVOU itoaMul100<>(SB), X11; \
	MOVOU itoaMul6554<>(SB), X12; \
	MOVOU itoaMul2559<>(SB), X13; \
	MOVOU itoaZeros<>(SB), X14; \
	PXOR X7, X7; \
	LEAQ itoaTables<>(SB), R12

#define XMM_DIGITS(v) \
	MOVQ v, X0; \
	PUNPCKLLQ X7, X0; \
	MOVO X0, X1; \
	PMULULQ X8, X1; \
	PSRLQ $45, X1; \
	MOVO X1, X2; \
	PMULULQ X9, X2; \
	PSUBQ X2, X0; \
	PSLLQ $32, X0; \
	POR X1, X0; \
	MOVO X0, X1; \
	PMULHUW X10, X1; \
	PSRLW $3, X1; \
	MOVO X1, X2; \
	PMULLW X11, X2; \
	PSUBW X2, X0; \
	PSLLL $16, X0; \
	POR X1, X0; \
	MOVO X0, X1; \
	PMULHUW X12, X1; \
	PSLLW $8, X0; \
	PMULLW X13, X1; \
	PSUBW X1, X0; \
	PADDB X14, X0

#define XMM_STORE(n) \
	MOVQ n, AX; \
	SHLQ $4, AX; \
	MOVOU SHUFFLE_BY_LEN(R12)(AX*1), X1; \
	PSHUFB X1, X0; \
	MOVOU X0, (DI); \
	ADDQ n, DI

#define XMM_ONE \
	DECIMAL_LEN(R8, BX); \
	SPLIT_PACK(R8); \
	XMM_DIGITS(R8); \
	XMM_STORE(BX)

#define XMM_WIDE \
	WIDE_SPLIT; \
	XMM_ONE; \
	SPLIT_PACK(R9); \
	XMM_DIGITS(R9); \
	MOVOU X0, (DI); \
	ADDQ $16, DI

#define YMM_CONSTANTS \
	VBROADCASTI128 itoaDiv10k<>(SB), Y8; \
	VBROADCASTI128 itoa10k<>(SB), Y9; \
	VBROADCASTI128 itoaMul5243<>(SB), Y10; \
	VBROADCASTI128 itoaMul100<>(SB), Y11; \
	VBROADCASTI128 itoaMul6554<>(SB), Y12; \
	VBROADCASTI128 itoaMul2559<>(SB), Y13; \
	VBROADCASTI128 itoaZeros<>(SB), Y14; \
	LEAQ itoaTables<>(SB), R12

#define YMM_DIGITS \
	VMOVQ R8, X0; \
	VPINSRQ $1, R9, X0, X0; \
	VPMOVZXDQ X0, Y0; \
	VPMULUDQ Y8, Y0, Y1; \
	VPSRLQ $45, Y1, Y1; \
	VPMULUDQ Y9, Y1, Y2; \
	VPSUBQ Y2, Y0, Y0; \
	VPSLLQ $32, Y0, Y0; \
	VPOR Y1, Y0, Y0; \
	VPMULHUW Y10, Y0, Y1; \
	VPSRLW $3, Y1, Y1; \
	VPMULLW Y11, Y1, Y2; \
	VPSUBW Y2, Y0, Y0; \
	VPSLLD $16, Y0, Y0; \
	VPOR Y1, Y0, Y0; \
	VPMULHUW Y12, Y0, Y1; \
	VPSLLW $8, Y0, Y0; \
	VPMULLW Y13, Y1, Y1; \
	VPSUBW Y1, Y0, Y0; \
	VPADDB Y14, Y0, Y0

#define YMM_SHUFFLE(lo, hi) \
	MOVQ lo, AX; \
	SHLQ $4, AX; \
	VMOVDQU SHUFFLE_BY_LEN(R12)(AX*1), X1; \
	MOVQ hi, AX; \
	SHLQ $4, AX; \
	VINSERTI128 $1, SHUFFLE_BY_LEN(R12)(AX*1), Y1, Y1; \
	VPSHUFB Y1, Y0, Y0

#define YMM_PAIR \
	DECIMAL_LEN(R8, BX); \
	DECIMAL_LEN(R9, R10); \
	SPLIT_PACK(R8); \
	SPLIT_PACK(R9); \
	YMM_DIGITS; \
	YMM_SHUFFLE(BX, R10)

#define YMM_ONE \
	DECIMAL_LEN(R8, BX); \
	SPLIT_PACK(R8); \
	XORL R9, R9; \
	YMM_DIGITS; \
	YMM_SHUFFLE(BX, BX); \
	VMOVDQU X0, (DI); \
	ADDQ BX, DI

#define YMM_WIDE \
	WIDE_SPLIT; \
	DECIMAL_LEN(R8, BX); \
	SPLIT_PACK(R8); \
	SPLIT_PACK(R9); \
	YMM_DIGITS; \
	MOVL $16, R10; \
	YMM_SHUFFLE(BX, R10); \
	VMOVDQU X0, (DI); \
	ADDQ BX, DI; \
	VEXTRACTI128 $1, Y0, (DI); \
	ADDQ $16, DI

TEXT ·formatInt64sSSSE3(SB), NOSPLIT, $0-32
	MOVQ dst+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), CX
	XMM_CONSTANTS
	TESTQ CX, CX
	JZ int64XmmDone

int64XmmLoop:
	MOVQ (SI), R8
	ABS(R8, R11)
	MOVB $0x2d, (DI)
	SUBQ R11, DI
	CMPQ R8, TEN16(R12)
	JAE int64XmmWide
	XMM_ONE

int64XmmNext:
	MOVB $0x2c, (DI)
	INCQ DI
	ADDQ $8, SI
	DECQ CX
	JNZ int64XmmLoop

int64XmmDone:
	SUBQ dst+0(FP), DI
	MOVQ DI, ret+24(FP)
	RET

int64XmmWide:
	XMM_WIDE
	JMP int64XmmNext

TEXT ·formatUint64sSSSE3(SB), NOSPLIT, $0-32
	MOVQ dst+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), CX
	XMM_CONSTANTS
	TESTQ CX, CX
	JZ uint64XmmDone

uint64XmmLoop:
	MOVQ (SI), R8
	CMPQ R8, TEN16(R12)
	JAE uint64XmmWide
	XMM_ONE

uint64XmmNext:
	MOVB $0x2c, (DI)
	INCQ DI
	ADDQ $8, SI
	DECQ CX
	JNZ uint64XmmLoop

uint64XmmDone:
	SUBQ dst+0(FP), DI
	MOVQ DI, ret+24(FP)
	RET

uint64XmmWide:
	XMM_WIDE
	JMP uint64XmmNext

TEXT ·formatInt64sAVX2(SB), NOSPLIT, $0-32
	MOVQ dst+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), CX
	YMM_CONSTANTS

int64YmmPairLoop:
	CMPQ CX, $2
	JB int64YmmSingle
	MOVQ (SI), R8
	MOVQ 8(SI), R9
	ABS(R8, R11)
	ABS(R9, R13)
	CMPQ R8, TEN16(R12)
	JAE int64YmmSingle
	CMPQ R9, TEN16(R12)
	JAE int64YmmSingle
	YMM_PAIR
	MOVB $0x2d, (DI)
	SUBQ R11, DI
	VMOVDQU X0, (DI)
	ADDQ BX, DI
	MOVB $0x2c, (DI)
	MOVB $0x2d, 1(DI)
	SUBQ R13, DI
	VEXTRACTI128 $1, Y0, 1(DI)
	LEAQ 1(DI)(R10*1), DI
	MOVB $0x2c, (DI)
	INCQ DI
	ADDQ $16, SI
	SUBQ $2, CX
	JMP int64YmmPairLoop

int64YmmSingle:
	TESTQ CX, CX
	JZ int64YmmDone
	MOVQ (SI), R8
	ABS(R8, R11)
	MOVB $0x2d, (DI)
	SUBQ R11, DI
	CMPQ R8, TEN16(R12)
	JAE int64YmmWide
	YMM_ONE

int64YmmSingleNext:
	MOVB $0x2c, (DI)
	INCQ DI
	ADDQ $8, SI
	DECQ CX
	JMP int64YmmPairLoop

int64YmmWide:
	YMM_WIDE
	JMP int64YmmSingleNext

int64YmmDone:
	VZEROUPPER
	SUBQ dst+0(FP), DI
	MOVQ DI, ret+24(FP)
	RET

TEXT ·formatUint64sAVX2(SB), NOSPLIT, $0-32
	MOVQ dst+0(FP), DI
	MOVQ src+8(FP), SI
	MOVQ n+16(FP), CX
	YMM_CONSTANTS

uint64YmmPairLoop:
	CMPQ CX, $2
	JB uint64YmmSingle
	MOVQ (SI), R8
	MOVQ 8(SI), R9
	CMPQ R8, TEN16(R12)
	JAE uint64YmmSingle
	CMPQ R9, TEN16(R12)
	JAE uint64YmmSingle
	YMM_PAIR
	VMOVDQU X0, (DI)
	ADDQ BX, DI
	MOVB $0x2c, (DI)
	VEXTRACTI128 $1, Y0, 1(DI)
	LEAQ 1(DI)(R10*1), DI
	MOVB $0x2c, (DI)
	INCQ DI
	ADDQ $16, SI
	SUBQ $2, CX
	JMP uint64YmmPairLoop

uint64YmmSingle:
	TESTQ CX, CX
	JZ uint64YmmDone
	MOVQ (SI), R8
	CMPQ R8, TEN16(R12)
	JAE uint64YmmWide
	YMM_ONE

uint64YmmSingleNext:
	MOVB $0x2c, (DI)
	INCQ DI
	ADDQ $8, SI
	DECQ CX
	JMP uint64YmmPairLoop

uint64YmmWide:
	YMM_WIDE
	JMP uint64YmmSingleNext

uint64YmmDone:
	VZEROUPPER
	SUBQ dst+0(FP), DI
	MOVQ DI, ret+24(FP)
	RET
