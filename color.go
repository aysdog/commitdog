package main

func colorRed(s string) string {
	return "\033[31m" + s + "\033[0m"
}

func colorGreen(s string) string {
	return "\033[32m" + s + "\033[0m"
}

func colorYellow(s string) string {
	return "\033[33m" + s + "\033[0m"
}

func colorDim(s string) string {
	return "\033[2m" + s + "\033[0m"
}

func colorMuted(s string) string {
	return "\033[90m" + s + "\033[0m"
}
