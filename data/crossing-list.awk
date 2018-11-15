BEGIN {
  etstart = "";
  etend = "";
  ststart = "";
  stend = "";
  eclipse = 0;
  saa = 0;
} {
  if ( eclipse == 0 && $6 == 1 ) {
    etstart = $1;
    eclipse = 1;
  }
  else if ( eclipse == 1 && $6 == 1 ) {
    etend = $1;
  }
  else if ( eclipse == 1 && $6 == 0 ) {
    printf("eclipse,%s,%s\n", etstart, etend);
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
    printf("saa,%s,%s\n", ststart, stend);
    saa = 0;
  }
}
