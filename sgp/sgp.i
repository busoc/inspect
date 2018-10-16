%module sgp
%include "typemaps.i"

%{
  #include "SGP4.h"
  using namespace SGP4Funcs;
%}

%rename(modifiedSGP4) sgp4;
bool sgp4(elsetrec& satrec, double tsince, double *INOUT, double *INOUT);

// rename TLE fields row#1
%rename(number) satnum;
%rename(classification) classification;
%rename(designation) intldesg;
%rename(year) epochyr;
%rename(days) epochdays;
%rename(mean1) ndot;
%rename(mean2) nddot;
%rename(bstar) bstar;
%rename(ephemeris) ephtype;

// rename TLE fields row#2
%rename(inclination) inclo;
%rename(ascension) nodeo;
%rename(excentricity) ecco;
%rename(perigee) argpo;
%rename(anomaly) mo;
%rename(motion) no_kozai;


%include "SGP4.h"
using namespace SGP4Funcs;

%go_import("fmt")

%insert(go_wrapper) %{
func SGP4(els Elsetrec, since float64) ([]float64, []float64, error) {
  ps := []float64{0.0, 0.0, 0.0}
  vs := []float64{0.0, 0.0, 0.0}

  ModifiedSGP4(els, since, ps, vs)
  switch els.GetError() {
  case 0:
  case 1:
    return nil, nil, fmt.Errorf("mean elements, ecc >= 1.0 or ecc < -0.001 or a < 0.95")
  case 2:
    return nil, nil, fmt.Errorf("mean motion less than 0.0")
  case 3:
    return nil, nil, fmt.Errorf("pert elements, ecc < 0.0  or  ecc > 1.0")
  case 4:
    return nil, nil, fmt.Errorf("semi-latus rectum < 0.0")
  case 5:
    return nil, nil, fmt.Errorf("epoch elements are sub-orbital")
  case 6:
    return nil, nil, fmt.Errorf("satellite has decayed")
  default:
    return nil, nil, fmt.Errorf("unrecognized error")
  }
  return ps, vs, nil
}
%}
