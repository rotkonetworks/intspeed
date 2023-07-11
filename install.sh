sudo cp speedtest-go /usr/local/bin
sudo chmod +x /usr/local/bin/speedtest-go
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
chmod +x intspeed.py
./intspeed.py
