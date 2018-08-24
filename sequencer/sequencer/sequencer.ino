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

#define Lband_PTT 4
#define Lband_50ohm_ref 5
#define Lband_event1 12

#define Sband_PTT 6
#define Sband_50ohm_ref 7
#define Sband_event1 13

#define Cband_PTT 8
#define Cband_50ohm_ref 9
#define Cband_event1 14

#define Xband_PTT 10
#define Xband_50ohm_ref 11
#define Xband_event1 15

// Output "coils":
// TX L, S, C, X
// REF L, S, C, X
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
  mb.config(&Serial1, MODBUS_BAUD, SERIAL_8N1, serial_tx_en);
  mb.setSlaveId(MODBUS_SLAVE_ID);

  for (int i = 0; i < BANDS; i++) {
    mb.addCoil(i, false);
    mb.addCoil(BANDS+i, false);
    mb.addIsts(i, false);
  }
}

void loop()
{
  mb.task();
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
  mb.Ists(0, invalid);
  if (invalid) {
    tx = -1;
  }
  for (int i = 0; i < BANDS; i++) {
    bool tx_confirm = digitalRead(INPUT_PIN_BASE+i);
    mb.Ists(1+i, tx_confirm);
    if (tx_confirm && i != tx) {
      // Previous band is still switching out of tx; don't try to key the next one until it finishes.
      tx = -1;
    }
  }
  for (int i = 0; i < BANDS; i++) {
    digitalWrite(OUTPUT_PIN_BASE+(i*2), i == tx);
    digitalWrite(OUTPUT_PIN_BASE+(i+2)+1, tx >= 0 || mb.Coil(BANDS+i));
  }
}

