BEGIN {
  radius = 6378.1363;
  excentricity = 0.006694385;
  flattening = 0.003352813178;
  pi = 3.14159265358979323846264338327950288419716939937510582097494459;
  deg2rad = pi / 180.0;
  row = "%5d || %12.6f | %12.5f | %12.5f | %12.5f || %12.6f | %12.5f | %12.5f | %12.5f || %12.5fkm\n"
  avg = 0
  min = 0
  max = 0
  latlon = 1
} {
  alt = $3;
  lat = $4 * deg2rad;
  lon = $5 * deg2rad;

  si = sin(lat) * sin(lat);
  n = radius * (1-flattening*(2-flattening)*si) ** -0.5;

  x0 = ((n+alt) * cos(lat) * cos(lon));
  y0 = ((n+alt) * cos(lat) * sin(lon));
  z0 = ((n*(1-excentricity) + alt) * sin (lat));

  alt = $10;
  lat = $11 * deg2rad;
  lon = $12 * deg2rad;

  si = sin(lat) * sin(lat);
  n = radius * (1-flattening*(2-flattening)*si) ** -0.5;

  x1 = ((n+alt) * cos(lat) * cos(lon));
  y1 = ((n+alt) * cos(lat) * sin(lon));
  z1 = ((n*(1-excentricity) + alt) * sin (lat));

  diff = ((x1-x0) ** 2) + ((y1-y0) ** 2) + ((z1-z0) ** 2)
  dist = sqrt(diff)
  avg += dist

  if (latlon==1) {
    printf(row, NR, $2, $3, $4, $5, $9, $10, $11, $12, dist)
  } else {
    printf(row, NR, $2, z0, x0, y0, $9, z1, x1, y1, dist)
  }
}
END {
  printf("average distance: %12.2fkm\n", avg/NR)
}
