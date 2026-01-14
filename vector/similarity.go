package vector

import "math"

func Dot(a,b []float32) float32{
	var sum float32 = 0

	for i := 0;i<len(a);i++{
		sum += a[i]*b[i]
	}

	return sum
}

func Magnitude(v []float32) float32{
	var sum float32 = 0;
	for i:= 0;i<len(v);i++{
		sum += v[i]*v[i]
	}

	return float32(math.Sqrt(float64(sum)))
}

func CosineSimilarity(a,b []float32) float32{
	dot := Dot(a,b)
	magA := Magnitude(a)
	magB := Magnitude(b)

	if magA ==0 || magB ==0{
		return 0
	}

	return dot/(magA*magB)
}




