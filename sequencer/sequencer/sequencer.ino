//dish RX/TX sequencer functios for the teensy.

#include <Modbus.h>
#include <ModbusSerial.h>

//pin definitions

//unused: #define serial_rx 0
//unused: #define serial_tx 1
#define serial_rx_en 2
#define serial_tx_en 3
#define MODBUS_SLAVE_ID 1
#define MODBUS_BAUD 19200

//#define Lband_PTT 4
//#define Lband_50ohm_ref 5
//#define Lband_event1 12

//#define Sband_PTT 6
//#define Sband_50ohm_ref 7
// NOTE: Pin 13 is also used by the onboard LED; high is ~2.85V with effective resistor divider.
//#define Sband_event1 13

//#define Cband_PTT 8
//#define Cband_50ohm_ref 9
//#define Cband_event1 14

//#define Xband_PTT 10
//#define Xband_50ohm_ref 11
//#define Xband_event1 15

// Output "coils":
// TX L, S, C, X
// RX L, S, C, X
// Input "descrete inputs":
// Invalid command
// L TX CONFIRM
// S TX CONFIRM
// C TX CONFIRM
// X TX CONFIRM

#define BANDS 4
#define OUTPUT_PIN_BASE 4
#define INPUT_PIN_BASE 12

ModbusSerial mb;

void setup()
{
  // Enable RX all the time.
  pinMode(serial_rx_en, OUTPUT);
  digitalWrite(serial_rx_en, LOW);

  // Configure a Modbus RTU slave on Serial1 (pins 1 and 2).
  mb.config(&Serial1, MODBUS_BAUD, SERIAL_8N1, serial_tx_en);
  // Modbus devices are addressed from 0 (master), 1-254 (slave), 255 (broadcast).
  mb.setSlaveId(MODBUS_SLAVE_ID);

  // Input register 0 contains the number of supported bands.
  mb.addIreg(0, BANDS);

  // Discrete input 0 shows fault status (e.g. multiple TX requested).
  mb.addIsts(0, false);
  // Create two output coils and one discrete input for each band.
  for (int i = 0; i < BANDS; i++) {
    mb.addCoil(i, false); // Coil i = TX request
    mb.addCoil(BANDS+i, false); // Coil BANDS+i = RX request
    mb.addIsts(1+i, false); // Discrete input 1+i = TX confirm

    // Set the sequencer pins to outputs and drive them inactive high.
    pinMode(OUTPUT_PIN_BASE+(i*2), OUTPUT);
    digitalWrite(OUTPUT_PIN_BASE+(i*2), HIGH);
    pinMode(OUTPUT_PIN_BASE+(i*2)+1, OUTPUT);
    digitalWrite(OUTPUT_PIN_BASE+(i*2)+1, HIGH);
    // Set the event1 input to an input.
    pinMode(INPUT_PIN_BASE+i, INPUT_PULLUP);
  }

  Serial.begin(9600); // for debugging
}

void loop()
{
  // Every time through the loop, we recompute the desired output
  // state from the current values of the Modbus coils.

  // Process any pending Modbus messages.
  mb.task();

  // Check if any of the TX request coils are set.
  bool invalid = false;
  int tx = -1;
  for (int i = 0; i < BANDS; i++) {
    if (mb.Coil(i)) {
      if (tx >= 0) {
        invalid = true;
      }
      tx = i;
    }
  }
  mb.Ists(0, invalid); // Expose on discrete input 0
  if (invalid) {
    tx = -1;
  }
  // tx = -1 if no TX request, or 0 to (BANDS-1) if a TX request.
  // Check the event1 inputs to see if any of the bands are already transmitting.
  for (int i = 0; i < BANDS; i++) {
    // event1 pins are active low.
    bool tx_confirm = !digitalRead(INPUT_PIN_BASE+i);
    mb.Ists(1+i, tx_confirm); // Expose on discrete input 1+i
    if (tx_confirm && i != tx) {
      // Previous band is still switching out of tx; don't try to key the next one until it finishes.
      tx = -1;
    }
  }

  for (int i = 0; i < BANDS; i++) {
    // Enable TX when band is selected. TX is active low.
    digitalWrite(OUTPUT_PIN_BASE+(i*2), !(i == tx));
    // Enable RX when requested AND there is no TX.
    digitalWrite(OUTPUT_PIN_BASE+(i*2)+1, !(tx < 0 && mb.Coil(BANDS+i)));
  }
}

