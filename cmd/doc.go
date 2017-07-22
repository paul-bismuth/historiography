/*

Histoctl rewrite git history to make it fits into times constraints.

You can rewrite multiple git repositories in one run by simply passing them
as command line arguments.

Usage:

  histoctl [flag] repo...

The flags are:

	--debug
		debug mode, equivalent to -vvvvv

	-v/--verbose
		verbose mode, multiple -v flag increase verbosity. The maximum is 5.

	-h/--help
		display help

	-f/--force
		apply changes to the repository without asking for validation or displaying
		new commit dates.

*/
package main
