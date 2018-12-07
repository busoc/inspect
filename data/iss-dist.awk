function distance() {
  alt = $3;
  lat = $4 * deg2rad;
  lon = $5 * deg2rad;

  si = sin(lat) * sin(lat);
  n = radius * (1-flattening*(2-flattening)*si) ** -0.5;

  x0 = ((n+alt) * cos(lat) * cos(lon));
  y0 = ((n+alt) * cos(lat) * sin(lon));
  z0 = ((n*(1-excentricity) + alt) * sin (lat));

  alt = $11;
  lat = $12 * deg2rad;
  lon = $13 * deg2rad;

  if (alt == 0 || lat == 0 || lon == 0) {
    return 0
  }

  si = sin(lat) * sin(lat);
  n = radius * (1-flattening*(2-flattening)*si) ** -0.5;

  x1 = ((n+alt) * cos(lat) * cos(lon));
  y1 = ((n+alt) * cos(lat) * sin(lon));
  z1 = ((n*(1-excentricity) + alt) * sin (lat));

  diff = ((x1-x0) ** 2) + ((y1-y0) ** 2) + ((z1-z0) ** 2)
  dist = sqrt(diff)

  printf(row, NR, $2, $3, $4, $5, $10, $11, $12, $13, dist)

  return dist
}

function ecefDistance() {
  diff = (($12-$4) ** 2) + (($13-$5) ** 2) + (($11-$3) ** 2)
  dist = sqrt(diff)

  return dist
}

BEGIN {
  radius = 6378.136;
  excentricity = 0.006694385;
  flattening = 0.003352813178;
  pi = 3.14159265358979323846264338327950288419716939937510582097494459;
  deg2rad = pi / 180.0;
  row = "%5d || %.5f | %12.5f | %12.5f | %12.5f || %.5f | %12.5f | %12.5f | %12.5f || %12.5fkm\n"
  avg = 0
  min = 0
  max = 0
  rc = 0
} {
  delta = $2-$10
  if (delta < 0) {
    delta = -delta
  }
  if (substr($1, 1, 19) == substr($9, 1, 19) || delta <= 0.00001) {
    rc++
    dist = 0
    if (mode == "ecef") {
      dist = ecefDistance()
    } else {
      dist = distance()
    }
    avg += dist
    printf(row, NR, $2, $3, $4, $5, $10, $11, $12, $13, dist)
  }
}
END {
  if (rc == 0) {
    rc++
  }
  printf("average distance: %12.2fkm\n", avg/rc)
}
