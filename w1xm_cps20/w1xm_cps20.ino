// -*- mode: c++; -*-
#include <P1AM.h>
#include <Modbus.h>
#include <ModbusSerial.h>

#define MODBUS_SLAVE_ID 20
#define MODBUS_BAUD 19200

#define RELAYS 2

// 5 seconds for time-delay relay, 3 seconds for spinup, plus safety margin
#define DELAY_SECONDS 15

// Output "coils":
// Azimuth, Elevation
// Input "discrete inputs":
// ALL CONFIRM
// AZ CONFIRM
// EL CONFIRM
// Input "input registers":
// # of relays
// "holding registers":
// confirmation delay

ModbusSerial mb;

void setup(){ // the setup routine runs once:
  pinMode(SWITCH_BUILTIN,INPUT);//Set our Switch (Pin 31) to be an input
  while (!P1.init()){ 
    ; //Wait for Modules to Sign on   
  }
  mb.config(&Serial, MODBUS_BAUD, SERIAL_8N1, -1);
  mb.setSlaveId(MODBUS_SLAVE_ID);

  // Input register 0 contains the number of relays.
  mb.addIreg(0, RELAYS);

  // Holding register 0 contains the confirmation delay in seconds.
  mb.addHreg(0, DELAY_SECONDS);

  // Discrete input 0 shows all relays on.
  mb.addIsts(0, false);

  // Create one output coil and one discrete input for each relay.
  for (int i = 0; i < RELAYS; i++) {
    mb.addCoil(i, false); // Coil i = Power on
    mb.addIsts(1+i, false); // Discrete input 1+i = Power confirm

    // Set the output to off.
    P1.writeDiscrete(false, 1, i+1);
  }
}

// millis wrap every 50 days.
unsigned long onMillis[RELAYS] = 0;

void loop(){  // the loop routine runs over and over again forever:
  bool switchState = digitalRead(SWITCH_BUILTIN);//Read the state of the switch

  unsigned long confirmation_delay = mb.Hreg(0);

  // Every time through the loop, we recompute the desired output
  // state from the current values of the Modbus coils.

  // Process any pending Modbus messages.
  mb.task();

  bool output[RELAYS];
  for (int i = 0; i < RELAYS; i++) {
    output[i] = mb.Coil(i) && switchState;
    P1.writeDiscrete(output[i], 1, 1+i);
    if (!output[i]) {
      // Off -> immediately disable confirmation.
      onMillis[i] = 0;
      mb.Ists(1+i, false);
    } else {
      if (onMillis[i] == 0) {
	onMillis[i] = millis();
      }
      if (!mb.Ists(1+i)) {
	// Check if enough time has elapsed.
	if (millis() > (onMillis[i]+confirmation_delay)) {
	  mb.Ists(1+i, true);
	} else if (millis() < onMillis[i]) {
	  // Clock has wrapped, restart the timer.
	  onMillis[i] = millis();
	}
      }
    }
  }
  bool all_on = true;
  for (int i = 0; i < RELAYS; i++) {
    all_on &&= mb.Ists(1+i);
  }
  mb.Ists(0, all_on); // Expose on discrete input 0
}
