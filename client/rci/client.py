import errno
import json
import time
import websocket
import os
import os.path
import sys
import urllib
from threading import Thread, Lock, Condition

class Client(object):
    def __init__(self, url=None, password=None):
        if not url:
            url = os.getenv("RCI_ADDRESS", "ws://localhost:8502/api/ws")
        if not password:
            password = os.getenv("RCI_PASSWORD")
        if '?' in url:
            url += '&'
        else:
            url += '?'
        url += 'client='+urllib.quote(os.path.basename(sys.argv[0]))
        self._url = url
        self._ws = websocket.WebSocket(enable_multithread=True)
        self._ws.connect(self._url, subprotocols=[password] if password else None)
        self._lock = Lock()
        self._cv = Condition(self._lock)
        self._status = {}
        t = Thread(target=self._recv_loop)
        t.daemon = True
        t.start()

    def _recv_loop(self):
        for message in self._ws:
            with self._cv:
                self._status = json.loads(message)
                self._cv.notifyAll()

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

    def set_azimuth_velocity(self, velocity):
        """Set azimuth velocity.

        Args:
            velocity: velocity in degrees/sec
        """
        self._send({
            'command': 'set_azimuth_velocity',
            'velocity': velocity,
        })

    def set_elevation_velocity(self, velocity):
        """Set elevation velocity.

        Args:
            velocity: velocity in degrees/sec
        """
        self._send({
            'command': 'set_elevation_velocity',
            'velocity': velocity,
        })

    def set_offsets(self, azimuth_offset, elevation_offset):
        """Set azimuth and elevation offsets.

        Args:
            azimuth_offset: angle in degrees to subtract from commanded azimuth
            elevation_offset: angle in degrees to subtract from commanded elevation
        """
        self._send({
        'command': 'set_azimuth_offset',
        'position': azimuth_offset,
        })
        self._send({
        'command': 'set_elevation_offset',
        'position': elevation_offset,
        })

    def stop(self):
        """Stop commanding movement."""
        self._send({
	    'command': 'stop',
        })

    def track(self, body):
        """Track a known body.

        Args:
            body: index of a body as returned by self.bodies
        """
        self._send({
	    'command': 'track',
	    'body': body,
        })

    def set_band_tx(self, band, enabled, wait=True, timeout=5):
        """Set a band to transmit.

        Args:
            band: 0-index of band to transmit on
            enabled: bool state
            wait: block until tx confirmation
            timeout: seconds to wait for tx confirmation
        """
        self._send({
            'command': 'set_band_tx',
            'band': band,
            'enabled': enabled,
        })
        if wait:
            start = time.time()
            with self._cv:
                while not self._status or not self._status['Sequencer']['Bands'][band]['TX']:
                    left=float(timeout)-(time.time()-start)
                    if left < 0:
                        raise IOError(errno.ETIMEDOUT, "timed out waiting for TX confirmation")
                    self._cv.wait(timeout=left)
                return

    def set_band_rx(self, band, enabled):
        """Set a band to receive.

        Args:
            band: 0-index of band to receive on
            enabled: bool state
        """
        self._send({
            'command': 'set_band_rx',
            'band': band,
            'enabled': enabled,
        })

    def i_know_what_i_am_doing_unsafe_exit_shutdown(self):
        """Exit the current shutdown state."""
        self._send({
            'command': 'exit_shutdown',
        })

    @property
    def azimuth_position(self):
        return self.status.get('AzPos')

    @property
    def azimuth_velocity(self):
        return self.status.get('AzVel')

    @property
    def elevation_position(self):
        return self.status.get('ElPos')

    @property
    def elevation_velocity(self):
        return self.status.get('ElVel')

    @property
    def bodies(self):
        """Return list of known bodies.

        The index of the body is passed to track(). If the list of
        bodies has not yet been received, returns None.

        Returns:
            List of strings
        """
        return self.status.get('Bodies')

    @property
    def status(self):
        """Returns the latest status dictionary."""
        with self._cv:
            while not self._status:
                self._cv.wait()
            return self._status

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
                'promodec': promodec,
                'parallax': parallax,
                'radialvelocity': radialvelocity,
            }
        })

if __name__ == "__main__":
    import time
    client = Client("ws://w1xm-radar-1.mit.edu:8502/api/ws")
    
    time.sleep(1)
    print client
