package vascula

import (
	"fmt"
	"math"
)

// Integers stay exact. Literals without a fractional part are int64, and
// arithmetic between two integers stays int64 unless it would overflow, when
// it falls back to float64 as before. Anything involving a float is float64.
// Output text is identical either way: an integral float prints without ".0".

// maxSafeInteger is the largest magnitude a float64 holds exactly.
const maxSafeInteger = 1 << 53

func toInt64(v interface{}) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	}
	return 0, false
}

// intPair reports both operands as integers when both are integer-typed.
func intPair(a, b interface{}) (int64, int64, bool) {
	x, ok := toInt64(a)
	if !ok {
		return 0, 0, false
	}
	y, ok := toInt64(b)
	return x, y, ok
}

func addInt(a, b int64) (int64, bool) {
	r := a + b
	if (a > 0 && b > 0 && r < 0) || (a < 0 && b < 0 && r >= 0) {
		return 0, false
	}
	return r, true
}

func subInt(a, b int64) (int64, bool) {
	if b == math.MinInt64 {
		return 0, false
	}
	return addInt(a, -b)
}

func mulInt(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if (a == -1 && b == math.MinInt64) || (b == -1 && a == math.MinInt64) {
		return 0, false
	}
	r := a * b
	if r/b != a {
		return 0, false
	}
	return r, true
}

// arithmetic builds plus/minus/times: exact for two integers, float otherwise.
func arithmetic(intOp func(a, b int64) (int64, bool), floatOp func(a, b float64) float64) FilterFunc {
	return func(in interface{}, args FilterArgs) (interface{}, error) {
		if x, y, ok := intPair(in, args.Arg(0)); ok {
			if r, ok := intOp(x, y); ok {
				return r, nil
			}
		}
		x, _ := toFloat(in)
		y, _ := toFloat(args.Arg(0))
		return floatOp(x, y), nil
	}
}

func dividedBy(in interface{}, args FilterArgs) (interface{}, error) {
	if x, y, ok := intPair(in, args.Arg(0)); ok {
		if y == 0 {
			return nil, fmt.Errorf("divided_by: division by zero")
		}
		// Division keeps its fractional result: 10 / 4 is 2.5, as before.
		if x%y == 0 && !(x == math.MinInt64 && y == -1) {
			return x / y, nil
		}
		return float64(x) / float64(y), nil
	}
	x, _ := toFloat(in)
	y, _ := toFloat(args.Arg(0))
	if y == 0 {
		return nil, fmt.Errorf("divided_by: division by zero")
	}
	return x / y, nil
}

func modulo(in interface{}, args FilterArgs) (interface{}, error) {
	if x, y, ok := intPair(in, args.Arg(0)); ok {
		if y == 0 {
			return nil, fmt.Errorf("modulo: division by zero")
		}
		if y == -1 {
			return int64(0), nil
		}
		return x % y, nil
	}
	x, _ := toFloat(in)
	y, _ := toFloat(args.Arg(0))
	if y == 0 {
		return nil, fmt.Errorf("modulo: division by zero")
	}
	return math.Mod(x, y), nil
}

func clamp(pickLarger bool) FilterFunc {
	return func(in interface{}, args FilterArgs) (interface{}, error) {
		if x, y, ok := intPair(in, args.Arg(0)); ok {
			if (y > x) == pickLarger {
				return y, nil
			}
			return x, nil
		}
		x, _ := toFloat(in)
		y, _ := toFloat(args.Arg(0))
		if pickLarger {
			return math.Max(x, y), nil
		}
		return math.Min(x, y), nil
	}
}

func absolute(in interface{}, _ FilterArgs) (interface{}, error) {
	if x, ok := toInt64(in); ok && x != math.MinInt64 {
		if x < 0 {
			return -x, nil
		}
		return x, nil
	}
	f, _ := toFloat(in)
	return math.Abs(f), nil
}

// hostValue hands a host filter the documented JSON shape: an integer a
// float64 holds exactly arrives as float64, as it always has. Larger integers
// stay int64 rather than arriving rounded.
func hostValue(v interface{}) interface{} {
	if x, ok := toInt64(v); ok && x <= maxSafeInteger && x >= -maxSafeInteger {
		return float64(x)
	}
	return v
}
