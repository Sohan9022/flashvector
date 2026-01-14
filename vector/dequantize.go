package vector

func Dequantize(qv QuantizedVector) []float32{
	q := make([]float32,len(qv.values))

	s := qv.scale

	for i,v := range qv.values{
		q[i] = float32(v)*s
	}

	return q
}