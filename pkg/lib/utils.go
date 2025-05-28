package lib

import (
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
)

func getFrame(skipFrames int) runtime.Frame {
	// We need the frame at index skipFrames+2, since we never want runtime.Callers and getFrame
	targetFrameIndex := skipFrames + 2

	// Set size to targetFrameIndex+2 to ensure we have room for one more caller than we need
	programCounters := make([]uintptr, targetFrameIndex+2)
	n := runtime.Callers(0, programCounters)

	frame := runtime.Frame{Function: "unknown"}
	if n > 0 {
		frames := runtime.CallersFrames(programCounters[:n])
		for more, frameIndex := true, 0; more && frameIndex <= targetFrameIndex; frameIndex++ {
			var frameCandidate runtime.Frame
			frameCandidate, more = frames.Next()
			if frameIndex == targetFrameIndex {
				frame = frameCandidate
			}
		}
	}

	return frame
}

func GetCurrentFunctionName() string {
	// Skip GetCurrentFunctionName
	fn := getFrame(1).Function
	if strings.Contains(fn, "(") {
		fn = "(" + strings.Split(fn, "(")[1]
	} else {
		p := strings.Split(fn, ".")
		fn = p[len(p)-1]
	}
	re := regexp.MustCompile(`(func\d+)`)
	if re.MatchString(fn) {
		fc := re.FindStringSubmatch(fn)[1]
		p := strings.Split(string(debug.Stack()), "\n")
		for _, v := range p {
			if strings.Contains(v, fc) {
				sp := strings.Split(v, "/")
				if len(sp) > 0 {
					v = sp[len(sp)-1:][0]
				}

				f := strings.Split(v, ".")
				if len(fn) > 1 {
					fn = strings.Join(f[1:], ".")
				} else {
					fn = v
				}
				break
			}
		}
	}
	return fn
}

func GetCallerFunctionName(opt ...int) string {
	f := 2
	if len(opt) > 0 {
		f += opt[0]
	}
	// Skip GetCallerFunctionName and the function to get the caller of
	fn := getFrame(f).Function
	if strings.Contains(fn, "(") {
		fn = "(" + strings.Split(fn, "(")[1]
	} else {
		p := strings.Split(fn, ".")
		fn = p[len(p)-1]
	}
	if fn == "goexit" {
		p := strings.Split(string(debug.Stack()), "\n")
		for _, v := range p {
			if strings.HasPrefix(v, "created by") {
				sp := strings.Split(v, "/")
				if len(sp) > 0 {
					v = sp[len(sp)-1:][0]
				}

				f := strings.Split(v, ".")
				if len(fn) > 1 {
					fn = strings.Join(f[1:], ".")
				} else {
					fn = v
				}
				break
			}
		}
	}
	return fn
}
