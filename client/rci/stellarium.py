#!/usr/bin/env python3

import asyncio
from twisted.internet import asyncioreactor
asyncioreactor.install(asyncio.get_event_loop())
from twisted.internet import reactor, protocol
from twisted.internet.task import react
from twisted.internet.defer import ensureDeferred
from twisted.protocols.basic import StringTooLongError
from twisted.python import log
import rci.client
from astropy.coordinates import ICRS, SkyCoord, EarthLocation, AltAz
import astropy.units as u
from astropy.time import Time
from collections import namedtuple
import math as m
import datetime
from datetime import timezone
import struct
import sys

RA_MAX = 0x100000000
DEC_MAX = 0x40000000
class _PropMixin:
    @property
    def time(self):
        return datetime.datetime.fromtimestamp(self.time_us / 1e6, timezone.utc)
    @time.setter
    def time(self, v):
        self.time_us = int(v.timestamp() * 1e6)

    @property
    def ra(self):
        return self.ra_int / RA_MAX * 360
    @ra.setter
    def ra(self, v):
        self.ra_int = int(v / 360 * RA_MAX)

    @property
    def dec(self):
        return self.dec_int / DEC_MAX * 90
    @dec.setter
    def dec(self, v):
        # Wrap to (-90,90)
        v = m.remainder(v, 180)
        self.dec_int = int(v / 90 * DEC_MAX)

    @property
    def packed(self):
        v = tuple(self)
        return struct.pack(self._format, *v)
    @classmethod
    def unpack(cls, b):
        return cls._make(struct.unpack(cls._format, b))

    def __iter__(self):
        return (getattr(self, value, 0) for value in self.__class__.__slots__)

    @classmethod
    def _make(cls, args):
        self = cls()
        for key, value in zip(cls.__slots__, args):
            setattr(self, key, value)
        return self

class MsgCurrentPosition(_PropMixin):
    __slots__ = "time_us ra_int dec_int status".split()

    TYPE = 0
    _format = "<qLll"
class MsgGoto(_PropMixin):
    __slots__ = "time_us ra_int dec_int".split()

    TYPE = 0
    _format = "<qLl"

class StellariumProtocol(protocol.Protocol):
    # Can't use IntNStringReceiver because the length includes the length bytes :(
    MAX_LENGTH = 9999
    structFormat = "<H"
    prefixLength = struct.calcsize(structFormat)
    _unprocessed = b""

    def dataReceived(self, data):
        alldata = self._unprocessed + data
        currentOffset = 0
        fmt = self.structFormat
        self._unprocessed = alldata

        while len(alldata) >= (currentOffset + self.prefixLength):
            messageStart = currentOffset + self.prefixLength
            (length,) = struct.unpack(fmt, alldata[currentOffset:messageStart])
            if length > self.MAX_LENGTH:
                self._unprocessed = alldata
                return
            messageEnd = messageStart + length - self.prefixLength
            if len(alldata) < messageEnd:
                break

            # Here we have to slice the working buffer so we can send just the
            # netstring into the stringReceived callback.
            packet = alldata[messageStart:messageEnd]
            currentOffset = messageEnd
            self.stringReceived(packet)

        # Slice off all the data that has been processed, avoiding holding onto
        # memory to store it.
        self._unprocessed = alldata[currentOffset:]

    def sendString(self, string):
        if len(string) >= 2 ** (8 * self.prefixLength):
            raise StringTooLongError(
                "Try to send %s bytes whereas maximum is %s"
                % (len(string), 2 ** (8 * self.prefixLength))
            )
        self.transport.write(struct.pack(self.structFormat, len(string) + self.prefixLength) + string)

    def stringReceived(self, string):
        if len(string) < 2:
            return
        pktType = struct.unpack("<H", string[:2])[0]
        if pktType == MsgGoto.TYPE:
            pkt = MsgGoto.unpack(string[2:])
            self.packetReceived(pkt)

    def packetReceived(self, pkt):
        raise NotImplementedError

    def sendPacket(self, pkt):
        self.sendString(struct.pack("<H", pkt.TYPE) + pkt.packed)

class StellariumRCIProtocol(StellariumProtocol):
    def connectionMade(self):
        self.factory.protocols.add(self)
        self.sendPacket(statusPacket(self.factory.client.status))

    def connectionLost(self, reason):
        self.factory.protocols.remove(self)

    def packetReceived(self, pkt):
        if isinstance(pkt, MsgGoto):
            log.msg("Received MsgGoto at time %s for (%f, %f)" % (pkt.time, pkt.ra, pkt.dec))
            status = self.factory.client.status
            loc = EarthLocation(lat=status["Latitude"], lon=status["Longitude"])
            frame = AltAz(location=loc, obstime=Time(pkt.time_us/1e6, format='unix'))
            sc = SkyCoord(ra=pkt.ra*u.deg, dec=pkt.dec*u.deg, frame='icrs')
            sca = sc.transform_to(frame)
            log.msg("Slewing to %s" % (sca,))
            self.factory.client.set_azimuth_position(sca.az.deg)
            self.factory.client.set_elevation_position(sca.alt.deg)

def statusPacket(status):
    loc = EarthLocation(lat=status["Latitude"], lon=status["Longitude"])
    frame = AltAz(location=loc, obstime=Time.now())
    sc = SkyCoord(az=status["AzPos"]*u.deg, alt=status["ElPos"]*u.deg, frame=frame)
    sci = sc.icrs
    msg = MsgCurrentPosition()
    msg.time = datetime.datetime.now()
    msg.ra = sci.ra.deg
    msg.dec = sci.dec.deg
    return msg

class StellariumRCIFactory(protocol.Factory):
    protocol = StellariumRCIProtocol

    def __init__(self, client):
        self.client = client
        self.protocols = set()
        reactor.callInThread(self.watchStatus)

    def watchStatus(self):
        for status in self.client:
            reactor.callFromThread(self.sendStatus, status)

    def sendStatus(self, status):
        msg = statusPacket(status)
        for protocol in self.protocols:
            protocol.sendPacket(msg)

async def _main(reactor):
    log.startLogging(sys.stderr)
    client = rci.client.Client(client_name='Stellarium')
    reactor.listenTCP(10001, StellariumRCIFactory(client))
    reactor.run()

def main():
    return react(
        lambda reactor: ensureDeferred(
            _main(reactor)
        )
    )
if __name__ == '__main__':
    main()
