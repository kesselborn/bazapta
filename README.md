Usage
====

Add a new package to 'squeezy' distribution on bazapta running on `localhost:5000`:
-----------------------------------------------------------------------------------

NOTE THE '@' BEFORE THE PATHNAME

    curl -i -XPOST -Ffile=@<path to deb package> localhost:5000/distributions/squeeze

List all 'squeezy' packages on bazapta running on `localhost:5000`:
-----------------------------------------------------------------------------------

    curl -i localhost:5000/distributions

Delete a package from 'squeezy' distribution on bazapta running on `localhost:5000`:
-----------------------------------------------------------------------------------

    curl -i -XDELETE localhost:5000/<path to package>

get the path to the package by listing packages

API routes
----------

1. .deb package built
2. POST /dists/squeezy/main
   req-entity: mypkg_1.0.0_i386.deb
   res-location: /dists/squeezy/main/mypkg_1.0.0_i386
3. metadata: GET /dists/[...]
4. package: GET /dists/[...].deb
5. remove: DELETE /dists/[...]

GET              /
GET|OPTIONS|POST /dists/:name(/:component)
GET|DELETE       /dists/:name/:component/:pkgname_:version_:arch
GET|PUT          /dists/:name/:component/:pkgname_:version_:arch.deb

GET /terms
GET /terms/DebianPackage
GET /terms/dist
GET /terms/component
GET /terms/arch
GET /terms/version
