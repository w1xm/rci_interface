#define PORT_WRITE_HIGH PORTD
#define PORT_WRITE_LOW PORTC
#define PORT_READ_HIGH PORTF
#define PORT_READ_LOW PORTB

#define PIN_HREQ PIN_E0
#define PIN_HREQ_ACTIVE LOW
#define PIN_HREQ_INACTIVE HIGH
#define PIN_HACK PIN_E1
#define PIN_HACK_ACTIVE LOW
#define PIN_HACK_INACTIVE HIGH
#define PIN_HCTL PIN_E6
#define PIN_HCTL_ACTIVE LOW
#define PIN_HCTL_INACTIVE HIGH
#define PIN_HREAD PIN_E7
#define PIN_HREAD_READ LOW
#define PIN_HREAD_WRITE HIGH

// Milliseconds to wait for HACK
#define TIMEOUT 1000

// We need at least 55; 10 registers * (4 num bytes + 1 space byte) + 1 address byte + 1 space + 1 carriage return + 1 newline + 1 null
char buf[82];
char* bufPtr = buf;

uint8_t hctl = LOW;

void setup() {
  memset(buf, 0, sizeof(buf));
  Serial.begin(9600);
  // Set read ports to inputs
  PINB = PINF = 0x00;
  // Set write ports to outputs
  PINC = PIND = 0xFF;
  pinMode(PIN_HREQ, OUTPUT);
  pinMode(PIN_HACK, INPUT);
  pinMode(PIN_HCTL, OUTPUT);
  digitalWrite(PIN_HCTL, hctl);
  pinMode(PIN_HREAD, OUTPUT);
  digitalWrite(PIN_HREAD, PIN_HREAD_READ);
  // Wait for inputs and outputs to settle
  delayMicroseconds(10);
}

void toggleHctl() {
  hctl ^= HIGH;
  digitalWrite(PIN_HCTL, hctl);
}

void doWrite() {
  long address = strtol(bufPtr, &bufPtr, 16);
  // Toggle HCTL to reset address
  digitalWrite(PIN_HREAD, PIN_HREAD_WRITE);
  toggleHctl();
  writeWord(address);
  char* endPtr = NULL;
  while (true) {
    long word = strtol(bufPtr, &endPtr, 16);
    if (endPtr == bufPtr) {
      break;
    }
    if (!writeWord(word)) {
      return;
    }
  }
}

bool writeWord(long word) {
  unsigned long start = millis();
  PORT_WRITE_LOW = word&0xFF;
  PORT_WRITE_HIGH = (word >> 8)&0xFF;
  // Assert HREQ
  digitalWrite(PIN_HREQ, PIN_HREQ_ACTIVE);
  // Wait for HACK
  while (digitalRead(PIN_HACK) != PIN_HACK_ACTIVE && (millis() - start) < TIMEOUT);
  if ((millis() - start) >= TIMEOUT) {
    digitalWrite(PIN_HREQ, PIN_HREQ_INACTIVE);
    Serial.print("\n! Write timed out waiting for HACK to become active\n");
    return false;
  }
  // Deassert HREQ
  digitalWrite(PIN_HREQ, PIN_HREQ_INACTIVE);
  while (digitalRead(PIN_HACK) != PIN_HACK_INACTIVE && (millis() - start) < TIMEOUT);
  if ((millis() - start) >= TIMEOUT) {
    Serial.print("\n! Write timed out waiting for HACK to become inactive\n");
    return false;
  }
  return true;
}

void loop() {
  // Read the full memory
  while (Serial.available()) {
    *bufPtr = Serial.read();
    bufPtr++;
    if (*(bufPtr-1) == '\n') {
      doWrite();
      memset(buf, 0, sizeof(buf));
      bufPtr = buf;
    }
  }
  unsigned long start = millis();
  digitalWrite(PIN_HREAD, PIN_HREAD_READ);
  // Toggle HCTL to reset address
  toggleHctl();
  for (int i = 0; i < 12; i++) {
    // Assert HREQ
    digitalWrite(PIN_HREQ, PIN_HREQ_ACTIVE);
    // Wait for HACK
    while (digitalRead(PIN_HACK) != PIN_HACK_ACTIVE && (millis() - start) < TIMEOUT);
    if ((millis() - start) >= TIMEOUT) {
      digitalWrite(PIN_HREQ, PIN_HREQ_INACTIVE);
      Serial.print("\n! Read timed out waiting for HACK to become active\n");
      return;
    }
    Serial.print(PORT_READ_HIGH, HEX);
    Serial.print(PORT_READ_LOW, HEX);
    // Deassert HREQ
    digitalWrite(PIN_HREQ, PIN_HREQ_INACTIVE);
    while (digitalRead(PIN_HACK) != PIN_HACK_INACTIVE && (millis() - start) < TIMEOUT);
    if ((millis() - start) >= TIMEOUT) {
      Serial.print("\n! Read timed out waiting for HACK to become inactive\n");
      return;
    }
    Serial.print(' ');
  }
  Serial.print('\n');
  Serial.send_now();
}
