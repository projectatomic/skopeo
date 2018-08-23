// Package slices has several utilities for operating on slices given Go's
// lack of generic types. Many functions take an argument of type func(l, r T)
// bool, that's expected to compute l < r where T is T in []T, the type of the
// given slice.
package slices
