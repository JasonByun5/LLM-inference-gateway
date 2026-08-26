// Package radix will hold a radix (compressed prefix) tree.
// Start with Insert / LongestPrefixMatch over byte slices, test-first.
//
// Built in rung 5; becomes the core of cache-aware routing, where it maps
// prompt-prefix blocks to the set of workers predicted to have them cached.
package radix
