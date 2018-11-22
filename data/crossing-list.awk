BEGIN {
  etstart = "";
  etend = "";
  ststart = "";
  stend = "";
  eclipse = 0;
  saa = 0;
  ecount = 0;
  scount = 0;
} {
  if ( eclipse == 0 && $6 == 1 ) {
    etstart = $1;
    eclipse = 1;
  }
  else if ( eclipse == 1 && $6 == 1 ) {
    etend = $1;
  }
  else if ( eclipse == 1 && $6 == 0 ) {
    ecount++
    printf("%3d | %-10s | %s | %s\n", ecount, "eclipse", etstart, etend);
    eclipse = 0;
  }

  if ( saa == 0 && $7 == 1 ) {
    ststart = $1;
    saa = 1;
  }
  else if ( saa == 1 && $7 == 1 ) {
    stend = $1;
  }
  else if ( saa == 1 && $7 == 0 ) {
    scount++
    printf("%3d | %-10s | %s | %s\n", scount, "saa", ststart, stend);
    saa = 0;
  }
}
