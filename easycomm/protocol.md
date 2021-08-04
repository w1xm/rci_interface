This controller and simulator speak a derivative of the Easycomm III protocol.
The upstream Protocol docs are available at
https://github.com/Hamlib/Hamlib/blob/master/rotators/easycomm/easycomm.txt When
the Easycomm III protocol did not have the necessary functionality, an attempt
was made to stick to the SATNOGS protocol at
https://gitlab.com/Quartapound/satnogs-rotator-firmware/-/blob/master/libraries/easycomm.h

Basic structure
-----------------

Commands are issued in ASCII and terminated by space, carriage return, and/or linefeed.

Computer -> rotor commands
-----------------

Commands issued without arguments are treated as a request for the rotor to report its current status (e.g. the `AZ` command will cause the rotor to return the current position as `AZ154.1`). Commands issued with arguments are treated as commands, and the rotor is not required to respond to any commands, even if successful.

Supported commands with arguments are as follows:

| Command | Description |
| ------- | ----------- |
| `AZnnn.n` | Put the azimuth axis into position servo and request a move to the given angle. **Extension** Two decimal places are also supported |
| `ELnnn.n` | Put the elevation axis into position servo and request a move to the given angle. **Extension** Two decimal places are also supported |
| `VLnnnn` / `VRnnnn` | Put the azimuth axis into velocity servo and request a move (left or right) at the given speed in millidegrees/second |
| `VUnnnn` / `VDnnnn` | Put the elevation axis into velocity servo and request a move (up or down) at the given speed in millidegrees/second |
| `SA` | Stop driving the azimuth axis (*Note* This is different from `VL0`, which will actively counteract motion) |
| `SE` | Stop driving the elevation axis (*Note* This is different from `VU0`, which will actively counteract motion) |
| `RESET` | **SATNOGS** Reset rotator, find home position (using limit switches) |
| `PARK` | **SATNOGS** Move to the parking position |
| `CWx,nnn` | Set config register |
| `CW1,nnn` | **SATNOGS** Set azimuth P |
| `CW2,nnn` | **SATNOGS** Set azimuth I |
| `CW3,nnn` | **SATNOGS** Set azimuth D |
| `CW4,nnn` | **SATNOGS** Set elevation P |
| `CW5,nnn` | **SATNOGS** Set elevation I |
| `CW6,nnn` | **SATNOGS** Set elevation D |
| `CW7,nnn` | **SATNOGS** Set azimuth park position |
| `CW8,nnn` | **SATNOGS** Set elevation park position |

Rotor -> computer status
-----

The rotor reports its status to the computer using the following responses,
which can either be solicited by the computer's request (`AZ` to request the
current azimuth position) or sent unsolicited as values change.

| Status | Description |
| ------ | ----------- |
| `AZnnn.n` | Current azimuth axis position in degrees. *Note* This is different from the commanded position. **Extension** Two decimal places are returned |
| `ELnnn.n` | Current elevation axis position in degrees. **Extension** Two decimal places are returned |
| `IP0,nnn.nn` | **SATNOGS** Current temperature
| `IP1,nnn` | **SATNOGS** Non-zero if azimuth is at end stop. **Extension** Treated as a bitfield, 1 indicating CCW limit and 2 indicating CW limit |
| `IP2,nnn` | **SATNOGS** Non-zero if elevation is at end stop. **Extension** Treated as a bitfield, 1 indicating lower limit and 2 indicating upper limit |
| `IP5,nnn` | **SATNOGS** Azimuth drive "load" |
| `IP6,nnn` | **SATNOGS** Elevation drive "load" |
| `IP7,nnn.n` | **SATNOGS** Azimuth velocity in degrees/second |
| `IP8,nnn.n` | **SATNOGS** Elevation velocity in degrees/second |
| `VEstring` | Rotor software version |
| `GSnnn` | Status register. Treated as bitmask, 1 = idle, 2 = moving, 4 = pointing, 8 = error **Extension** 6 (4+2) indicates position servo mode, 2 indicates velocity servo mode. Treated as a 16-bit number, azimuth status is in lower 8 bits, elevation status is in upper 8 bits |
| `GEnnn` | Error register. Treated as bitmask, **SATNOGS** 1 = no error, 2 = sensor error, 4 = homing error, 8 = motor error |
| `CRn,nnn.n` | (See `CW` commands above) |
| `CR10,nnn.n` | **Extension** Current commanded azimuth position (as set by last `AZnnn.nn` command) |
| `CR11,nnn.n` | **Extension** Current commanded elevation position (as set by last `ELnnn.nn` command) |
| `CR12,nnn.n` | **Extension** Current commanded azimuth velocity (as set by last `VRnnn.nn` command) |
| `CR13,nnn.n` | **Extension** Current commanded elevation velocity (as set by last `VUnnn.nn` command) |
| `\?ENCnnn,nnn,nnn,nnn` | **Extension** Current raw encoder values in ticks (azimuth position, elevation position, azimuth velocity, elevation velocity) |
