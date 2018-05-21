import json
import websocket
from threading import Thread, Lock

class Client(object):
    def __init__(self, url):
        self._url = url
        self._ws = websocket.WebSocket(enable_multithread=True)
        self._ws.connect(self._url)
        self._lock = Lock()
        self._status = {}
        t = Thread(target=self._recv_loop)
        t.daemon = True
        t.start()

    def _recv_loop(self):
        for message in self._ws:
            with self._lock:
                self._status = json.loads(message)

    def _send(self, message):
        self._ws.send(json.dumps(message))

    def __str__(self):
        with self._lock:
            return str(self._status)

    def close(self):
        """Close connection."""
        with self._lock:
            self._ws.close()

    def set_azimuth_position(self, position):
        """Set azimuth position.

        Args:
            position: angle in degrees
        """
        self._send({
	    'command': 'set_azimuth_position',
	    'position': position,
	})

    def set_elevation_position(self, position):
        """Set azimuth position.

        Args:
            position: angle in degrees
        """
        self._send({
	    'command': 'set_elevation_position',
	    'position': position,
        })


    def stop(self):
        """Stop commanding movement."""
        self._send({
	    'command': 'stop',
        })

    def track(self, body):
        """Track a known body.

        Args:
            body: index of a body as returned by bodies()
        """
        self._send({
	    'command': 'track',
	    'body': body,
        })

    def bodies(self):
        """Return list of known bodies.

        The index of the body is passed to track(). If the list of
        bodies has not yet been received, returns None.

        Returns:
            List of strings
        """
        with self._lock:
            if 'Bodies' in self._status:
                return self._status['Bodies']
            return None

    def add_star(self, starname, catalog, starnumber, ra, dec, promora, promodec, parallax, radialvelocity):
        """Add a star to the list of known bodies.

        Args:
	    starname: name of celestial object
            catalog: catalog designator (e.g., HIP)
            starnumber: integer identifier assigned to object
            ra: ICRS right ascension (hours)
            dec: ICRS declination (degrees)
            promora: ICRS proper motion in right ascension (milliarcseconds/year)
            promodec: ICRS proper motion in declination (milliarcseconds/year)
            parallax: parallax (milliarcseconds)
            radialvelocity: radial velocity (km/s)
        """
        self._send({
            'command': 'add_star',
            'star': {
                'starname': starname,
                'catalog': catalog,
                'starnumber': starnumber,
                'ra': ra,
                'dec': dec,
                'promora': promora,
                'promodec': prmodec,
                'parallax': parallax,
                'radialvelocity': radicalvelocity,
            }
        })

if __name__ == "__main__":
    import time
    client = Client("ws://w1xm-radar-1.mit.edu:8502/api/ws")
    
    time.sleep(1)
    print client
