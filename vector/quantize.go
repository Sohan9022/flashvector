package vector

import "math"

func Quantize(vec []float32) QuantizedVector{
	if len(vec) == 0 {
		panic("cannot quantize empty-vector")
	}

	// find max abs value
	var maxabs float32 = 0

	for _,v := range vec{
		abs := float32(math.Abs(float64(v)))

		if abs > maxabs{
			maxabs = abs
		}
	}

	// avoid div by 0

	if maxabs == 0{
		maxabs = 1
	}

	scale := maxabs / 127.0

	// quantize each value

	qvals := make([]int8,len(vec))

	for i,v := range vec{
	q := int(math.Round(float64(v/scale)))

	if q>127{
		q = 127
	}
	if q < -128{
		q = -128
	}

	qvals[i] = int8(q)
	}

	return QuantizedVector{
	values : qvals,
	scale : scale,
}

}



