Usage
====

Add a new package to 'squeezy' distribution on bazapta running on `localhost:5000`:
-----------------------------------------------------------------------------------

    curl -i -XPOST -Ffile=@<path to deb package> localhost:5000/distributions/squeeze

List all 'squeezy' packages on bazapta running on `localhost:5000`:
-----------------------------------------------------------------------------------

    curl -i localhost:5000/distributions

Delete a package from 'squeezy' distribution on bazapta running on `localhost:5000`:
-----------------------------------------------------------------------------------

    curl -i -XDELETE localhost:5000/<path to package>

get the path to the package by listing packages
