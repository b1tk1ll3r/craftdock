package main

// Minimal QR code encoder (byte mode, error correction level M, versions
// 1–10) – enough for otpauth:// URIs, without external dependencies.

import (
	"errors"
	"fmt"
	"strings"
)

type qrVersion struct {
	ecPerBlock int
	groups     [][2]int // {blocks, data codewords per block}
	align      []int
}

var qrVersionsM = []qrVersion{
	1:  {10, [][2]int{{1, 16}}, nil},
	2:  {16, [][2]int{{1, 28}}, []int{6, 18}},
	3:  {26, [][2]int{{1, 44}}, []int{6, 22}},
	4:  {18, [][2]int{{2, 32}}, []int{6, 26}},
	5:  {24, [][2]int{{2, 43}}, []int{6, 30}},
	6:  {16, [][2]int{{4, 27}}, []int{6, 34}},
	7:  {18, [][2]int{{4, 31}}, []int{6, 22, 38}},
	8:  {22, [][2]int{{2, 38}, {2, 39}}, []int{6, 24, 42}},
	9:  {22, [][2]int{{3, 36}, {2, 37}}, []int{6, 26, 46}},
	10: {26, [][2]int{{4, 43}, {1, 44}}, []int{6, 28, 50}},
}

func (v qrVersion) dataCodewords() int {
	n := 0
	for _, g := range v.groups {
		n += g[0] * g[1]
	}
	return n
}

type qrCode struct {
	size    int
	modules [][]bool
	isFunc  [][]bool
}

func qrEncode(text string) (*qrCode, error) {
	data := []byte(text)
	ver := 0
	for v := 1; v <= 10; v++ {
		countBits := 8
		if v >= 10 {
			countBits = 16
		}
		if 4+countBits+8*len(data) <= qrVersionsM[v].dataCodewords()*8 {
			ver = v
			break
		}
	}
	if ver == 0 {
		return nil, errors.New("QR: Text zu lang")
	}
	vi := qrVersionsM[ver]

	// Bit stream
	var bits []bool
	put := func(val, n int) {
		for i := n - 1; i >= 0; i-- {
			bits = append(bits, (val>>i)&1 == 1)
		}
	}
	put(0b0100, 4)
	if ver >= 10 {
		put(len(data), 16)
	} else {
		put(len(data), 8)
	}
	for _, b := range data {
		put(int(b), 8)
	}
	capBits := vi.dataCodewords() * 8
	for i := 0; i < 4 && len(bits) < capBits; i++ {
		bits = append(bits, false)
	}
	for len(bits)%8 != 0 {
		bits = append(bits, false)
	}
	for pad := 0; len(bits) < capBits; pad++ {
		if pad%2 == 0 {
			put(0xEC, 8)
		} else {
			put(0x11, 8)
		}
	}
	cw := make([]byte, len(bits)/8)
	for i, b := range bits {
		if b {
			cw[i/8] |= 1 << (7 - uint(i%8))
		}
	}

	// Blocks + Reed-Solomon
	div := rsDivisor(vi.ecPerBlock)
	var blocks, ecBlocks [][]byte
	pos, maxLen := 0, 0
	for _, g := range vi.groups {
		for i := 0; i < g[0]; i++ {
			blk := cw[pos : pos+g[1]]
			pos += g[1]
			blocks = append(blocks, blk)
			ecBlocks = append(ecBlocks, rsRemainder(blk, div))
			if g[1] > maxLen {
				maxLen = g[1]
			}
		}
	}
	var final []byte
	for i := 0; i < maxLen; i++ {
		for _, b := range blocks {
			if i < len(b) {
				final = append(final, b[i])
			}
		}
	}
	for i := 0; i < vi.ecPerBlock; i++ {
		for _, b := range ecBlocks {
			final = append(final, b[i])
		}
	}

	q := &qrCode{size: ver*4 + 17}
	q.modules = make([][]bool, q.size)
	q.isFunc = make([][]bool, q.size)
	for i := range q.modules {
		q.modules[i] = make([]bool, q.size)
		q.isFunc[i] = make([]bool, q.size)
	}
	q.drawFunctionPatterns(ver, vi.align)
	q.drawCodewords(final)

	best, bestScore := 0, -1
	for mask := 0; mask < 8; mask++ {
		q.applyMask(mask)
		q.drawFormat(mask)
		if s := q.penalty(); bestScore < 0 || s < bestScore {
			best, bestScore = mask, s
		}
		q.applyMask(mask) // undo (XOR)
	}
	q.applyMask(best)
	q.drawFormat(best)
	return q, nil
}

func (q *qrCode) set(x, y int, dark bool) {
	q.modules[y][x] = dark
	q.isFunc[y][x] = true
}

func (q *qrCode) drawFunctionPatterns(ver int, align []int) {
	for i := 0; i < q.size; i++ {
		q.set(6, i, i%2 == 0)
		q.set(i, 6, i%2 == 0)
	}
	for _, c := range [][2]int{{3, 3}, {q.size - 4, 3}, {3, q.size - 4}} {
		for dy := -4; dy <= 4; dy++ {
			for dx := -4; dx <= 4; dx++ {
				x, y := c[0]+dx, c[1]+dy
				if x < 0 || y < 0 || x >= q.size || y >= q.size {
					continue
				}
				d := max(abs(dx), abs(dy))
				q.set(x, y, d != 2 && d != 4)
			}
		}
	}
	n := len(align)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if (i == 0 && j == 0) || (i == 0 && j == n-1) || (i == n-1 && j == 0) {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					q.set(align[i]+dx, align[j]+dy, max(abs(dx), abs(dy)) != 1)
				}
			}
		}
	}
	q.drawFormat(0) // reserve
	if ver >= 7 {
		rem := ver
		for i := 0; i < 12; i++ {
			rem = (rem << 1) ^ ((rem >> 11) * 0x1F25)
		}
		bits := ver<<12 | rem
		for i := 0; i < 18; i++ {
			bit := (bits>>i)&1 == 1
			a, b := q.size-11+i%3, i/3
			q.set(a, b, bit)
			q.set(b, a, bit)
		}
	}
}

func (q *qrCode) drawFormat(mask int) {
	data := 0<<3 | mask // level M = 00
	rem := data
	for i := 0; i < 10; i++ {
		rem = (rem << 1) ^ ((rem >> 9) * 0x537)
	}
	bits := (data<<10 | rem) ^ 0x5412
	bit := func(i int) bool { return (bits>>i)&1 == 1 }
	for i := 0; i <= 5; i++ {
		q.set(8, i, bit(i))
	}
	q.set(8, 7, bit(6))
	q.set(8, 8, bit(7))
	q.set(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		q.set(14-i, 8, bit(i))
	}
	for i := 0; i < 8; i++ {
		q.set(q.size-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		q.set(8, q.size-15+i, bit(i))
	}
	q.set(8, q.size-8, true)
}

func (q *qrCode) drawCodewords(data []byte) {
	i := 0
	for right := q.size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		for vert := 0; vert < q.size; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				y := vert
				if (right+1)&2 == 0 {
					y = q.size - 1 - vert
				}
				if !q.isFunc[y][x] && i < len(data)*8 {
					q.modules[y][x] = (data[i>>3]>>(7-uint(i&7)))&1 == 1
					i++
				}
			}
		}
	}
}

func (q *qrCode) applyMask(mask int) {
	for y := 0; y < q.size; y++ {
		for x := 0; x < q.size; x++ {
			if q.isFunc[y][x] {
				continue
			}
			var inv bool
			switch mask {
			case 0:
				inv = (x+y)%2 == 0
			case 1:
				inv = y%2 == 0
			case 2:
				inv = x%3 == 0
			case 3:
				inv = (x+y)%3 == 0
			case 4:
				inv = (x/3+y/2)%2 == 0
			case 5:
				inv = x*y%2+x*y%3 == 0
			case 6:
				inv = (x*y%2+x*y%3)%2 == 0
			case 7:
				inv = ((x+y)%2+x*y%3)%2 == 0
			}
			if inv {
				q.modules[y][x] = !q.modules[y][x]
			}
		}
	}
}

// penalty implements the scoring rules of ISO 18004 §8.8.2 (slightly
// simplified finder-pattern rule) to pick a well-scannable mask.
func (q *qrCode) penalty() int {
	n, score := q.size, 0
	get := func(x, y int, horiz bool) bool {
		if horiz {
			return q.modules[y][x]
		}
		return q.modules[x][y]
	}
	for _, horiz := range []bool{true, false} {
		for a := 0; a < n; a++ {
			run := 1
			for b := 1; b < n; b++ {
				if get(b, a, horiz) == get(b-1, a, horiz) {
					run++
					if run == 5 {
						score += 3
					} else if run > 5 {
						score++
					}
				} else {
					run = 1
				}
			}
			for b := 0; b+11 <= n; b++ {
				var s strings.Builder
				for k := 0; k < 11; k++ {
					if get(b+k, a, horiz) {
						s.WriteByte('1')
					} else {
						s.WriteByte('0')
					}
				}
				if p := s.String(); p == "10111010000" || p == "00001011101" {
					score += 40
				}
			}
		}
	}
	dark := 0
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if q.modules[y][x] {
				dark++
			}
			if x < n-1 && y < n-1 {
				c := q.modules[y][x]
				if c == q.modules[y][x+1] && c == q.modules[y+1][x] && c == q.modules[y+1][x+1] {
					score += 3
				}
			}
		}
	}
	k := (abs(dark*20-n*n*10) + n*n - 1) / (n * n)
	if k > 0 {
		score += (k - 1) * 10
	}
	return score
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func gfMul(x, y byte) byte {
	z := 0
	for i := 7; i >= 0; i-- {
		z = (z << 1) ^ ((z >> 7) * 0x11D)
		z ^= int((y>>uint(i))&1) * int(x)
	}
	return byte(z)
}

func rsDivisor(degree int) []byte {
	res := make([]byte, degree)
	res[degree-1] = 1
	root := byte(1)
	for i := 0; i < degree; i++ {
		for j := 0; j < degree; j++ {
			res[j] = gfMul(res[j], root)
			if j+1 < degree {
				res[j] ^= res[j+1]
			}
		}
		root = gfMul(root, 0x02)
	}
	return res
}

func rsRemainder(data, div []byte) []byte {
	res := make([]byte, len(div))
	for _, b := range data {
		factor := b ^ res[0]
		copy(res, res[1:])
		res[len(res)-1] = 0
		for i := range res {
			res[i] ^= gfMul(div[i], factor)
		}
	}
	return res
}

// SVG renders the code with a 4-module quiet zone.
func (q *qrCode) SVG() string {
	const border = 4
	dim := q.size + 2*border
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges"><rect width="100%%" height="100%%" fill="#fff"/><path fill="#000" d="`, dim, dim)
	for y := 0; y < q.size; y++ {
		for x := 0; x < q.size; x++ {
			if q.modules[y][x] {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x+border, y+border)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}
