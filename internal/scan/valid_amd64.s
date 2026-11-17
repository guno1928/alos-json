//go:build amd64 && !purego

#include "go_asm.h"
#include "textflag.h"

#include "strclass_amd64.h"

DATA vldCaseFold<>+0(SB)/8, $0x2020202020202020
DATA vldCaseFold<>+8(SB)/8, $0x2020202020202020
DATA vldCaseFold<>+16(SB)/8, $0x2020202020202020
DATA vldCaseFold<>+24(SB)/8, $0x2020202020202020
GLOBL vldCaseFold<>(SB), (NOPTR+RODATA), $32

DATA vldOpTable<>+0(SB)/8, $0xFFFFFFFFFFFFFFFF
DATA vldOpTable<>+8(SB)/8, $0xFFFF7D2C7B3AFFFF
DATA vldOpTable<>+16(SB)/8, $0xFFFFFFFFFFFFFFFF
DATA vldOpTable<>+24(SB)/8, $0xFFFF7D2C7B3AFFFF
GLOBL vldOpTable<>(SB), (NOPTR+RODATA), $32

DATA vldSpaceTable<>+0(SB)/8, $0xFFFFFFFFFFFFFF20
DATA vldSpaceTable<>+8(SB)/8, $0xFFFF0DFFFF0A09FF
DATA vldSpaceTable<>+16(SB)/8, $0xFFFFFFFFFFFFFF20
DATA vldSpaceTable<>+24(SB)/8, $0xFFFF0DFFFF0A09FF
GLOBL vldSpaceTable<>(SB), (NOPTR+RODATA), $32

#define QUOTE_MASKS(data) \
	VPCMPEQB Y15, data, Y2; \
	VPCMPEQB Y14, data, Y3

#define STRUCT_MASKS(data) \
	VPOR     Y13, data, Y4; \
	VPSHUFB  Y4, Y12, Y5; \
	VPCMPEQB Y5, Y4, Y5; \
	VPSHUFB  data, Y11, Y6; \
	VPCMPEQB Y6, data, Y6; \
	VPOR     Y5, Y6, Y6

#define MERGE_HIGH(vec, dst) \
	VPMOVMSKB vec, DX; \
	SHLQ      $32, DX; \
	ORQ       DX, dst

#define CLASS(data, dst, tmp) \
	VPAND   Y2, data, dst; \
	VPSHUFB dst, Y4, dst; \
	VPSRLW  $4, data, tmp; \
	VPAND   Y2, tmp, tmp; \
	VPSHUFB tmp, Y6, tmp; \
	VPAND   tmp, dst, dst

#define CLASS_MASK(lo, hi, op, dst) \
	op(lo); \
	VPMOVMSKB Y5, dst; \
	op(hi); \
	MERGE_HIGH(Y5, dst)

#define BAD_ESCAPE(cls) \
	VPAND    Y10, cls, Y5; \
	VPCMPEQB Y2, Y5, Y5

#define UNICODE_ESCAPE(cls) \
	VPSLLW $3, cls, Y5

#define NON_HEX(cls) \
	VPAND    strHexBits<>(SB), cls, Y5; \
	VPCMPEQB Y2, Y5, Y5

#define FLATTEN_ONE(off) \
	TZCNTQ R10, DX; \
	ADDL   R13, DX; \
	MOVL   DX, off(DI); \
	BLSRQ  R10, R10

TEXT ·validBlocksStrict(SB), NOSPLIT, $0-48
	MOVQ    p+0(FP), SI
	MOVQ    n+8(FP), CX
	ADDQ    SI, CX
	MOVQ    st+16(FP), BX
	MOVQ    out+24(FP), DI
	MOVQ    outCap+32(FP), R14
	SUBQ    $64, R14
	LEAQ    (DI)(R14*4), R14
	VMOVDQU strQuote<>(SB), Y15
	VMOVDQU strBslash<>(SB), Y14
	VMOVDQU vldCaseFold<>(SB), Y13
	VMOVDQU vldOpTable<>(SB), Y12
	VMOVDQU vldSpaceTable<>(SB), Y11
	VMOVDQU strEscBits<>(SB), Y10
	VPCMPEQB Y9, Y9, Y9

block:
	CMPQ  SI, CX
	JAE   exit
	CMPQ  DI, R14
	JA    exit
	MOVQ  blockState_inStrMask(BX), AX
	NOTQ  AX
	ORQ   blockState_escCarry(BX), AX
	ORQ   blockState_hexCarry(BX), AX
	JNZ   load

	CMPB  blockState_lax(BX), $0
	JNE   laxStringRun

stringRun:
	VMOVDQU   (SI), Y0
	VMOVDQU   32(SI), Y1
	VPCMPEQB  Y15, Y0, Y2
	VPCMPEQB  Y14, Y0, Y3
	VPMINUB   Y10, Y0, Y4
	VPCMPEQB  Y4, Y0, Y4
	VPCMPEQB  Y15, Y1, Y5
	VPCMPEQB  Y14, Y1, Y6
	VPMINUB   Y10, Y1, Y7
	VPCMPEQB  Y7, Y1, Y7
	VPOR      Y2, Y3, Y2
	VPOR      Y4, Y5, Y4
	VPOR      Y6, Y7, Y6
	VPOR      Y2, Y4, Y2
	VPOR      Y2, Y6, Y2
	VPTEST    Y2, Y2
	JNZ       classify
	ADDQ      $64, SI
	CMPQ      SI, CX
	JB        stringRun
	JMP       exit

load:
	VMOVDQU (SI), Y0
	VMOVDQU 32(SI), Y1
	CMPB    blockState_lax(BX), $0
	JNE     laxClassify

classify:
	VPMINUB   Y10, Y0, Y7
	VPCMPEQB  Y7, Y0, Y7
	VPMINUB   Y10, Y1, Y8
	VPCMPEQB  Y8, Y1, Y8
	QUOTE_MASKS(Y0)
	VPMOVMSKB Y2, R8
	VPMOVMSKB Y3, R9
	QUOTE_MASKS(Y1)
	MERGE_HIGH(Y2, R8)
	MERGE_HIGH(Y3, R9)
	XORL      R12, R12
	VPOR      Y7, Y8, Y2
	VPTEST    Y2, Y2
	JZ        quotes
	VPMOVMSKB Y7, R12
	MERGE_HIGH(Y8, R12)

quotes:
	TESTQ     R8, R8
	JNZ       structure
	CMPQ      blockState_inStrMask(BX), $0
	JL        escapes

structure:
	STRUCT_MASKS(Y0)
	VPMOVMSKB Y5, R10
	VPMOVMSKB Y6, R11
	STRUCT_MASKS(Y1)
	MERGE_HIGH(Y5, R10)
	MERGE_HIGH(Y6, R11)
	ORQ       R8, R11

escapes:
	MOVQ  blockState_escCarry(BX), AX
	MOVQ  AX, DX
	ORQ   blockState_hexCarry(BX), DX
	ORQ   R9, DX
	JZ    strings
	ANDNQ R9, AX, R9
	LEAQ  (AX)(R9*2), DX
	ANDNQ R9, DX, AX
	MOVQ  $0xAAAAAAAAAAAAAAAA, R13
	ANDQ  R13, AX
	ADDQ  AX, R9
	SBBQ  AX, AX
	NEGQ  AX
	MOVQ  AX, blockState_escCarry(BX)
	ADDQ  R9, R9
	MOVQ  $0x5555555555555555, R13
	XORQ  R13, R9
	ANDQ  DX, R9
	ANDNQ R8, R9, R8
	CMPB  blockState_lax(BX), $0
	JNE   laxStrings

	VMOVDQU strNibble<>(SB), Y2
	VMOVDQU strClassLo<>(SB), Y4
	VMOVDQU strClassHi<>(SB), Y6
	CLASS(Y0, Y3, Y5)
	CLASS(Y1, Y7, Y5)
	VPXOR   Y2, Y2, Y2

	CLASS_MASK(Y3, Y7, BAD_ESCAPE, AX)
	TESTQ R9, AX
	JNZ   fail

	CLASS_MASK(Y3, Y7, UNICODE_ESCAPE, AX)
	ANDQ  R9, AX
	LEAQ  (AX)(AX*2), DX
	MOVQ  DX, R13
	SHLQ  $2, R13
	ORQ   DX, R13
	ADDQ  R13, R13
	ORQ   blockState_hexCarry(BX), R13
	SHRQ  $58, AX
	ANDL  $0x3C, AX
	MOVQ  $0xFFFFFFFF77773310, DX
	SHRXQ AX, DX, DX
	ANDL  $15, DX
	MOVQ  DX, blockState_hexCarry(BX)

	CLASS_MASK(Y3, Y7, NON_HEX, AX)
	TESTQ R13, AX
	JNZ   fail

strings:
	VMOVQ      R8, X2
	VPCLMULQDQ $0, X9, X2, X2
	VMOVQ      X2, AX
	MOVQ       blockState_inStrMask(BX), DX
	SARQ       $63, DX
	XORQ       DX, AX
	MOVQ       AX, blockState_inStrMask(BX)
	TESTQ      AX, R12
	JNZ        fail
	ANDNQ      R10, AX, R10
	ANDQ       AX, R8
	ORQ        R8, R10
	ORQ        AX, R11
	NOTQ       R11
	LEAQ       (R11)(R11*1), DX
	ORQ        blockState_scalarCarry(BX), DX
	ANDNQ      R11, DX, DX
	ORQ        DX, R10
	SHRQ       $63, R11
	MOVQ       R11, blockState_scalarCarry(BX)
	TESTQ      R10, R10
	JZ         advance

	MOVQ    SI, R13
	SUBQ    p+0(FP), R13
	ADDQ    blockState_pos(BX), R13
	POPCNTQ R10, R12
	MOVQ    DI, AX

flatten:
	FLATTEN_ONE(0)
	FLATTEN_ONE(4)
	FLATTEN_ONE(8)
	FLATTEN_ONE(12)
	ADDQ  $16, DI
	TESTQ R10, R10
	JNZ   flatten
	LEAQ  (AX)(R12*4), DI

advance:
	ADDQ $64, SI
	JMP  block

laxStringRun:
	VMOVDQU  (SI), Y0
	VMOVDQU  32(SI), Y1
	VPCMPEQB Y15, Y0, Y2
	VPCMPEQB Y14, Y0, Y3
	VPCMPEQB Y15, Y1, Y5
	VPCMPEQB Y14, Y1, Y6
	VPOR     Y2, Y3, Y2
	VPOR     Y5, Y6, Y5
	VPOR     Y2, Y5, Y2
	VPTEST   Y2, Y2
	JNZ      laxClassify
	ADDQ     $64, SI
	CMPQ     SI, CX
	JB       laxStringRun
	JMP      exit

laxStrings:
	TESTQ R8, R8
	JNZ   strings
	CMPQ  blockState_inStrMask(BX), $0
	JGE   strings
	MOVQ  $-1, blockState_inStrMask(BX)
	MOVQ  blockState_escCarry(BX), R12

laxInString:
	ADDQ      $64, SI
	CMPQ      SI, CX
	JAE       laxInStringDone
	VMOVDQU   (SI), Y0
	VMOVDQU   32(SI), Y1
	VPCMPEQB  Y15, Y0, Y2
	VPCMPEQB  Y15, Y1, Y3
	VPCMPEQB  Y14, Y0, Y4
	VPCMPEQB  Y14, Y1, Y5
	VPMOVMSKB Y2, R8
	VPMOVMSKB Y3, DX
	SHLQ      $32, DX
	ORQ       DX, R8
	VPMOVMSKB Y4, R9
	VPMOVMSKB Y5, DX
	SHLQ      $32, DX
	ORQ       DX, R9
	MOVQ      R12, R13
	MOVQ      R12, AX
	ORQ       R9, AX
	JZ        laxInQuotes
	MOVQ      R12, AX
	ANDNQ     R9, AX, R9
	LEAQ      (AX)(R9*2), DX
	ANDNQ     R9, DX, AX
	MOVQ      $0xAAAAAAAAAAAAAAAA, R10
	ANDQ      R10, AX
	ADDQ      AX, R9
	SBBQ      R12, R12
	NEGQ      R12
	ADDQ      R9, R9
	MOVQ      $0x5555555555555555, R10
	XORQ      R10, R9
	ANDQ      DX, R9
	ANDNQ     R8, R9, R8

laxInQuotes:
	TESTQ R8, R8
	JZ    laxInString
	MOVQ  DI, R11

laxCloseQuote:
	TZCNTQ  R8, DX
	CMPQ    DX, $60
	JA      laxBail
	MOVWLZX 1(SI)(DX*1), AX
	CMPL    AX, $0x222C
	JNE     laxBail
	CMPQ    DI, R14
	JA      laxBail
	MOVQ    SI, R10
	SUBQ    p+0(FP), R10
	ADDQ    blockState_pos(BX), R10
	LEAQ    1(R10)(DX*1), R10
	MOVL    R10, (DI)
	INCL    R10
	MOVL    R10, 4(DI)
	ADDQ    $8, DI
	LEAL    3(DX), R10
	MOVQ    $-1, AX
	SHLXQ   R10, AX, AX
	ANDQ    AX, R8
	JNZ     laxCloseQuote
	JMP     laxInString

laxBail:
	MOVQ R11, DI
	MOVQ R13, blockState_escCarry(BX)
	JMP  load

laxInStringDone:
	MOVQ R12, blockState_escCarry(BX)
	JMP  exit

laxClassify:
	QUOTE_MASKS(Y0)
	VPMOVMSKB Y2, R8
	VPMOVMSKB Y3, R9
	QUOTE_MASKS(Y1)
	MERGE_HIGH(Y2, R8)
	MERGE_HIGH(Y3, R9)
	XORL      R12, R12
	JMP       quotes

exit:
	SUBQ p+0(FP), SI
	ADDQ SI, blockState_pos(BX)
	SUBQ out+24(FP), DI
	SHRQ $2, DI
	VZEROUPPER
	MOVQ DI, ret+40(FP)
	RET

fail:
	VZEROUPPER
	MOVQ $-1, ret+40(FP)
	RET
