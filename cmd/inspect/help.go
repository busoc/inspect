package main

const helpText = `Satellite trajectory prediction tool with Eclipse and SAA crossing.

Usage: inspect [-c] [-d] [-i] [-f] [-r] [-s] [-t] [-w] [-360] [-dms] <tle,...>

inspect calculates the trajectory of a given satellite from a set of (local or
remote) TLE (two-line elements set). To predict the path of a satellite, it uses
the SGP4 library written by D. Vallado in C++.

The predicted trajectory given by inspect computes each point independantly from
the previous, unlike other propagation methods.

Coordinate systems/frames:

inspect can give the position of a satellite in three different way (mutually
exclusive):

- geocentric: the latitude, longitude and altitude are calculated from the centre
of the earth.

- geodetic: the latitude, longitude and altitude are calculated above an ellipsoidal
surface of the earth.

- teme/eci: the latitude, longitude and altitude are calculated from the centre of the
earth. The main difference is that, in this frame, the values are computed in an
inertial system that do not rotate with the earth. These values are the outcome
of the SGP4 propagator used by inspect and are used the computed the latitude,
longitude in the geodetic or geocentric frame.

TLE/Input format:

inspect can only support the following TLE format (the first line being optional.
But if present should be 24 characters long)

ISS (ZARYA)
1 25544U 98067A   18304.35926896  .00001207  00000-0  25703-4 0  9995
2 25544  51.6420  60.1332 0004268 356.0118  61.1534 15.53880871139693

Output format:


the output of inspect consists of a tabulated "file". The columns of the file are:

- datetime (YYYY-mm-dd HH:MM:SS.ssssss)
- modified julian day
- altitude (kilometer)
- latitude (degree or DMS)
- longitude (degree or DMS)
- eclipse (1: night, 0: day)
- crossing (1: crossing, 0: no crossing)
- TLE epoch (not printed when output is pipe separated)

Options:

  -b       DATE    start date
  -c       COORD   coordinate system used (geocentric, geodetic, teme/eci)
  -d       TIME    TIME over which calculate the predicted trajectory
  -f       FORMAT  print predicted trajectory in FORMAT (csv, pipe, json, xml)
  -i       TIME    TIME between two points on the predicted trajectory
  -r       AREA    check if the predicted trajectory crossed the given AREA
  -s       SID     satellite identifier
  -t       DIR     store a TLE fetched from a remote server in DIR
  -w       FILE    write predicted trajectory in FILE (default to stdout)
  -bstar   LIMIT   B-STAR drag coefficient limit
  -360             longitude are given in range of [0:360[ instead of ]-180:180[
  -dms             convert latitude and longitude to DD°MIN'SEC'' format
  -config          load settings from a configuration file
  -version         print inspect version and exit
  -info            print info about the given TLE
  -help            print this message and exit

Examples:

# calculate the predicted trajectory over 24h for the default satellite from the
# latest TLE available on celestrak
$ inspect -d 24h -i 10s https://celestrak.com/NORAD/elements/stations.txt

# calculate the predicted trajectory over 24h for the default satellite from a
# locale TLE
$ inspect -c geodetic -dms -d 72h -i 1m /tmp/tle-201481119.txt

# calculate the predicted trajectory over 72h for the default satellite with 1 minute
# between two points of the path. The positions will be computed according to the
# geodetic system and printed as DD°MM'SS'. Moreover, it will check if the satellite
# cross a rectangle draw above a small town in Belgium.
$ inspect -r 51.0:46.0:49.0:50 -c geodetic -dms -d 72h -i 1m /tmp/tle-201481119.txt

# use a configuration file instead of command line options
$ inspect -config etc/inspect.toml
`
