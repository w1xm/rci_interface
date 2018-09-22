#!/usr/bin/env python
# -*- coding: utf-8 -*-
# 
# Copyright 2018 <+YOU OR YOUR COMPANY+>.
# 
# This is free software; you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation; either version 3, or (at your option)
# any later version.
# 
# This software is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
# 
# You should have received a copy of the GNU General Public License
# along with this software; see the file COPYING.  If not, write to
# the Free Software Foundation, Inc., 51 Franklin Street,
# Boston, MA 02110-1301, USA.
# 

import numpy as np
from gnuradio import gr
from rci import client

class pos_max_hold_fvf(gr.sync_block):
    """
    docstring for block pos_max_hold_fvf
    """
    def __init__(self, ws_url="ws://localhost:8502/api/ws", buckets=360):
        gr.sync_block.__init__(self,
            name="pos_max_hold_fvf",
            in_sig=[np.float32],
            out_sig=[(np.float32, buckets)]
        )
        self._client = client.Client(ws_url)
        self._buckets = buckets
        self._alpha = 0.9
        self._max_hold = np.zeros(buckets, np.float64)
        self._max_hold -= 140

    def work(self, input_items, output_items):
        input_item = max(input_items[0])
        input_item = 10 * np.log10(1000*input_item)
        az = self._client.status['AzPos']
        #print az, input_item
        bucket = int(az/360*self._buckets)
        self._max_hold[bucket] = (self._max_hold[bucket]*self._alpha) + (input_item*(1-self._alpha))
        output_items[0][0][:] = self._max_hold
        return len(output_items[0])
