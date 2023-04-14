xml2csv
-------

XML is a pain, and the existing Go libraries for dealing with it are a sad lot.

Here we convert .xml to .csv (comma separated values) for sane further processing.

This code will parse an XML file on stdin, and write csv to stdout. No schema required.
You do not need to create structs in Go first; no data-specific structs are involved.
There is just one `tag` struct used to process the .xml, and everything ends up in
a tree of them.

It was written for a specific need, and is not polished at all. It assumes that the
repeated records of interest are at depth one, and turns each of these into a row in the CSV.

Feel free to fork and adapt it to your own needs. I'll probably not do further work on it, but
maybe it can be the starting point for something of yours.


Copyright (c) 2023 Jason E. Aten, Ph.D.

License: MIT
