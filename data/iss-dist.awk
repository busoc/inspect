function distance(latlon) {
  alt = $3;
  lat = $4 * deg2rad;
  lon = $5 * deg2rad;

  si = sin(lat) * sin(lat);
  n = radius * (1-flattening*(2-flattening)*si) ** -0.5;

  x0 = ((n+alt) * cos(lat) * cos(lon));
  y0 = ((n+alt) * cos(lat) * sin(lon));
  z0 = ((n*(1-excentricity) + alt) * sin (lat));

  alt = $12;
  lat = $13 * deg2rad;
  lon = $14 * deg2rad;

  si = sin(lat) * sin(lat);
  n = radius * (1-flattening*(2-flattening)*si) ** -0.5;

  x1 = ((n+alt) * cos(lat) * cos(lon));
  y1 = ((n+alt) * cos(lat) * sin(lon));
  z1 = ((n*(1-excentricity) + alt) * sin (lat));

  diff = ((x1-x0) ** 2) + ((y1-y0) ** 2) + ((z1-z0) ** 2)
  dist = sqrt(diff)
  avg += dist

  if (latlon==1) {
    printf(row, NR, $1, $3, $4, $5, $10, $12, $13, $14, dist)
  } else {
    printf(row, NR, $2, z0, x0, y0, $9, z1, x1, y1, dist)
  }
}

BEGIN {
  radius = 6378.1363;
  excentricity = 0.006694385;
  flattening = 0.003352813178;
  pi = 3.14159265358979323846264338327950288419716939937510582097494459;
  deg2rad = pi / 180.0;
  row = "%5d || %s | %9.5f | %9.5f | %9.5f || %s | %9.5f | %9.5f | %9.5f || %9.5fkm\n"
  # row = "%5d || %12.6f | %12.5f | %12.5f | %12.5f || %12.6f | %12.5f | %12.5f | %12.5f || %12.5fkm\n"
  avg = 0
  min = 0
  max = 0
  latlon = 1
} {
  if (substr($1, 1, 19) == substr($10, 1, 19)) {
    distance(latlon)
  }
}
END {
  printf("average distance: %12.2fkm\n", avg/NR)
}
