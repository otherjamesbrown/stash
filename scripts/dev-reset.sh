#!/bin/bash
# dev-reset.sh - Build stash and create a test instance with sample data
#
# Usage: ./scripts/dev-reset.sh
#
# This script:
# 1. Builds the stash binary
# 2. Drops the existing 'world' test stash (if exists)
# 3. Creates a new 'world' stash with countries and cities
#
# The test stash uses:
# - Prefix: wld-
# - Top-level records: 10 largest countries by population
# - Child records: 10 largest cities per country

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
STASH="$PROJECT_DIR/stash"
TESTDATA="$PROJECT_DIR/testdata"

cd "$PROJECT_DIR"

echo "=== Building stash ==="
go build -o stash ./cmd/stash

echo "=== Dropping existing test stash (if any) ==="
$STASH drop world --yes 2>/dev/null || true

echo "=== Creating world stash ==="
$STASH init world --prefix wld- --no-daemon

echo "=== Adding columns ==="
$STASH column add Name --desc "Country or city name"
$STASH column add Population --desc "Population in millions"
$STASH column add Overview --desc "Link to overview markdown file"
# City-specific columns
$STASH column add Founded --desc "Year founded or established"
$STASH column add Elevation --desc "Elevation in meters"
$STASH column add Area --desc "Area in square kilometers"

echo "=== Adding countries and cities ==="

# Function to add a country and its cities
add_country() {
    local name="$1"
    local pop="$2"
    local slug="$3"

    echo "  Adding $name..."
    local country_id=$($STASH add "$name" \
        --set "Population=$pop" \
        --set "Overview=testdata/countries/${slug}.md")

    # Return the country ID for adding cities
    echo "$country_id"
}

add_city() {
    local parent_id="$1"
    local name="$2"
    local pop="$3"
    local founded="$4"
    local elevation="$5"
    local area="$6"
    local slug="$7"

    $STASH add "$name" \
        --parent "$parent_id" \
        --set "Population=$pop" \
        --set "Founded=$founded" \
        --set "Elevation=$elevation" \
        --set "Area=$area" \
        --set "Overview=testdata/cities/${slug}.md" > /dev/null
}

# China (1,425 million)
china=$($STASH add "China" --set "Population=1425" --set "Overview=testdata/countries/china.md")
add_city "$china" "Shanghai" "29.2" "751" "4" "6341" "shanghai"
add_city "$china" "Beijing" "21.5" "1045" "44" "16411" "beijing"
add_city "$china" "Chongqing" "16.3" "316" "244" "82403" "chongqing"
add_city "$china" "Tianjin" "13.9" "1404" "5" "11917" "tianjin"
add_city "$china" "Guangzhou" "13.5" "214" "21" "7434" "guangzhou"
add_city "$china" "Shenzhen" "12.9" "1979" "0" "1998" "shenzhen"
add_city "$china" "Chengdu" "11.3" "316" "500" "14335" "chengdu"
add_city "$china" "Nanjing" "9.4" "495" "20" "6587" "nanjing"
add_city "$china" "Wuhan" "11.1" "1500" "37" "8494" "wuhan"
add_city "$china" "Xi'an" "9.6" "202" "405" "10752" "xian"

# India (1,428 million)
india=$($STASH add "India" --set "Population=1428" --set "Overview=testdata/countries/india.md")
add_city "$india" "Mumbai" "21.0" "1347" "14" "603" "mumbai"
add_city "$india" "Delhi" "32.9" "1052" "216" "1484" "delhi"
add_city "$india" "Bangalore" "13.2" "1537" "920" "709" "bangalore"
add_city "$india" "Hyderabad" "10.5" "1591" "505" "650" "hyderabad"
add_city "$india" "Ahmedabad" "8.4" "1411" "53" "505" "ahmedabad"
add_city "$india" "Chennai" "11.5" "1639" "6" "426" "chennai"
add_city "$india" "Kolkata" "15.1" "1690" "9" "205" "kolkata"
add_city "$india" "Surat" "7.8" "1306" "13" "327" "surat"
add_city "$india" "Pune" "7.4" "1436" "560" "331" "pune"
add_city "$india" "Jaipur" "4.1" "1727" "431" "485" "jaipur"

# United States (339 million)
usa=$($STASH add "United States" --set "Population=339" --set "Overview=testdata/countries/usa.md")
add_city "$usa" "New York" "8.3" "1624" "10" "783" "new-york"
add_city "$usa" "Los Angeles" "3.9" "1781" "71" "1213" "los-angeles"
add_city "$usa" "Chicago" "2.7" "1833" "176" "606" "chicago"
add_city "$usa" "Houston" "2.3" "1837" "15" "1658" "houston"
add_city "$usa" "Phoenix" "1.6" "1881" "331" "1341" "phoenix"
add_city "$usa" "Philadelphia" "1.6" "1682" "12" "369" "philadelphia"
add_city "$usa" "San Antonio" "1.5" "1718" "198" "1307" "san-antonio"
add_city "$usa" "San Diego" "1.4" "1769" "19" "842" "san-diego"
add_city "$usa" "Dallas" "1.3" "1841" "131" "997" "dallas"
add_city "$usa" "Austin" "1.0" "1839" "149" "828" "austin"

# Indonesia (277 million)
indonesia=$($STASH add "Indonesia" --set "Population=277" --set "Overview=testdata/countries/indonesia.md")
add_city "$indonesia" "Jakarta" "10.6" "1527" "8" "662" "jakarta"
add_city "$indonesia" "Surabaya" "2.9" "1293" "5" "374" "surabaya"
add_city "$indonesia" "Bandung" "2.5" "1810" "768" "168" "bandung"
add_city "$indonesia" "Medan" "2.4" "1590" "26" "265" "medan"
add_city "$indonesia" "Semarang" "1.8" "1547" "3" "374" "semarang"
add_city "$indonesia" "Makassar" "1.5" "1607" "5" "199" "makassar"
add_city "$indonesia" "Palembang" "1.7" "683" "8" "401" "palembang"
add_city "$indonesia" "Tangerang" "1.9" "1943" "14" "164" "tangerang"
add_city "$indonesia" "Depok" "2.5" "1999" "77" "200" "depok"
add_city "$indonesia" "Bekasi" "2.5" "1945" "19" "210" "bekasi"

# Pakistan (231 million)
pakistan=$($STASH add "Pakistan" --set "Population=231" --set "Overview=testdata/countries/pakistan.md")
add_city "$pakistan" "Karachi" "16.1" "1729" "8" "3780" "karachi"
add_city "$pakistan" "Lahore" "13.5" "1040" "217" "1772" "lahore"
add_city "$pakistan" "Faisalabad" "3.7" "1880" "184" "1230" "faisalabad"
add_city "$pakistan" "Rawalpindi" "2.3" "1849" "508" "259" "rawalpindi"
add_city "$pakistan" "Gujranwala" "2.3" "1849" "226" "75" "gujranwala"
add_city "$pakistan" "Peshawar" "2.3" "1500" "331" "1257" "peshawar"
add_city "$pakistan" "Multan" "2.1" "1500" "123" "133" "multan"
add_city "$pakistan" "Hyderabad" "2.0" "1768" "13" "292" "hyderabad-pk"
add_city "$pakistan" "Islamabad" "1.2" "1960" "507" "906" "islamabad"
add_city "$pakistan" "Quetta" "1.1" "1876" "1680" "2653" "quetta"

# Nigeria (223 million)
nigeria=$($STASH add "Nigeria" --set "Population=223" --set "Overview=testdata/countries/nigeria.md")
add_city "$nigeria" "Lagos" "15.9" "1472" "41" "1171" "lagos"
add_city "$nigeria" "Kano" "4.4" "1000" "472" "499" "kano"
add_city "$nigeria" "Ibadan" "3.6" "1829" "230" "3080" "ibadan"
add_city "$nigeria" "Abuja" "3.6" "1991" "840" "1769" "abuja"
add_city "$nigeria" "Port Harcourt" "3.2" "1912" "12" "360" "port-harcourt"
add_city "$nigeria" "Benin City" "1.8" "1180" "80" "1204" "benin-city"
add_city "$nigeria" "Maiduguri" "1.2" "1907" "354" "543" "maiduguri"
add_city "$nigeria" "Zaria" "1.1" "1536" "660" "300" "zaria"
add_city "$nigeria" "Aba" "1.0" "1901" "60" "70" "aba"
add_city "$nigeria" "Jos" "0.9" "1915" "1217" "291" "jos"

# Brazil (216 million)
brazil=$($STASH add "Brazil" --set "Population=216" --set "Overview=testdata/countries/brazil.md")
add_city "$brazil" "Sao Paulo" "12.4" "1554" "760" "1521" "sao-paulo"
add_city "$brazil" "Rio de Janeiro" "6.7" "1565" "11" "1200" "rio-de-janeiro"
add_city "$brazil" "Brasilia" "3.0" "1960" "1172" "5802" "brasilia"
add_city "$brazil" "Salvador" "2.9" "1549" "8" "693" "salvador"
add_city "$brazil" "Fortaleza" "2.7" "1726" "16" "314" "fortaleza"
add_city "$brazil" "Belo Horizonte" "2.5" "1897" "852" "331" "belo-horizonte"
add_city "$brazil" "Manaus" "2.2" "1669" "92" "11401" "manaus"
add_city "$brazil" "Curitiba" "1.9" "1693" "934" "435" "curitiba"
add_city "$brazil" "Recife" "1.6" "1537" "4" "218" "recife"
add_city "$brazil" "Porto Alegre" "1.5" "1772" "10" "497" "porto-alegre"

# Bangladesh (172 million)
bangladesh=$($STASH add "Bangladesh" --set "Population=172" --set "Overview=testdata/countries/bangladesh.md")
add_city "$bangladesh" "Dhaka" "22.5" "1608" "4" "306" "dhaka"
add_city "$bangladesh" "Chittagong" "5.3" "1340" "0" "155" "chittagong"
add_city "$bangladesh" "Khulna" "1.0" "1884" "9" "46" "khulna"
add_city "$bangladesh" "Rajshahi" "0.9" "1634" "18" "96" "rajshahi"
add_city "$bangladesh" "Sylhet" "0.7" "1303" "35" "27" "sylhet"
add_city "$bangladesh" "Rangpur" "0.4" "1700" "34" "44" "rangpur"
add_city "$bangladesh" "Comilla" "0.6" "1790" "9" "15" "comilla"
add_city "$bangladesh" "Gazipur" "2.3" "1998" "14" "329" "gazipur"
add_city "$bangladesh" "Narayanganj" "0.8" "1876" "8" "67" "narayanganj"
add_city "$bangladesh" "Tongi" "0.9" "1964" "10" "34" "tongi"

# Russia (144 million)
russia=$($STASH add "Russia" --set "Population=144" --set "Overview=testdata/countries/russia.md")
add_city "$russia" "Moscow" "12.6" "1147" "156" "2511" "moscow"
add_city "$russia" "Saint Petersburg" "5.6" "1703" "3" "1439" "saint-petersburg"
add_city "$russia" "Novosibirsk" "1.6" "1893" "150" "505" "novosibirsk"
add_city "$russia" "Yekaterinburg" "1.5" "1723" "270" "468" "yekaterinburg"
add_city "$russia" "Kazan" "1.3" "1005" "116" "425" "kazan"
add_city "$russia" "Nizhny Novgorod" "1.2" "1221" "100" "411" "nizhny-novgorod"
add_city "$russia" "Chelyabinsk" "1.2" "1736" "219" "530" "chelyabinsk"
add_city "$russia" "Samara" "1.1" "1586" "44" "541" "samara"
add_city "$russia" "Omsk" "1.1" "1716" "90" "567" "omsk"
add_city "$russia" "Rostov-on-Don" "1.1" "1749" "73" "354" "rostov-on-don"

# Mexico (128 million)
mexico=$($STASH add "Mexico" --set "Population=128" --set "Overview=testdata/countries/mexico.md")
add_city "$mexico" "Mexico City" "9.2" "1325" "2240" "1485" "mexico-city"
add_city "$mexico" "Guadalajara" "1.5" "1542" "1566" "151" "guadalajara"
add_city "$mexico" "Monterrey" "1.1" "1596" "540" "324" "monterrey"
add_city "$mexico" "Puebla" "1.7" "1531" "2135" "534" "puebla"
add_city "$mexico" "Tijuana" "1.9" "1889" "20" "637" "tijuana"
add_city "$mexico" "Leon" "1.6" "1576" "1815" "1219" "leon"
add_city "$mexico" "Juarez" "1.5" "1659" "1137" "321" "juarez"
add_city "$mexico" "Zapopan" "1.5" "1542" "1574" "893" "zapopan"
add_city "$mexico" "Ecatepec" "1.6" "1620" "2259" "155" "ecatepec"
add_city "$mexico" "Nezahualcoyotl" "1.1" "1963" "2235" "63" "nezahualcoyotl"

echo ""
echo "=== Done! ==="
echo ""
$STASH info
echo ""
echo "Try these commands:"
echo "  ./stash list                    # List countries"
echo "  ./stash list --all              # List all (countries + cities)"
echo "  ./stash list --parent wld-xxx   # List cities in a country"
echo "  ./stash show <id>               # Show record details"
