module example.com/testproj

go 1.21

require github.com/example/extpkg v0.0.0

replace github.com/example/extpkg => ./stub/extpkg
