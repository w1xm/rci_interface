#define WRITE_HIGH C
#define WRITE_LOW D
#define READ_HIGH F
#define READ_LOW B

#define PORT_WRITE_HIGH _CONCATENATE(PORT, WRITE_HIGH)
#define DDR_WRITE_HIGH _CONCATENATE(DDR, WRITE_HIGH)
#define PORT_WRITE_LOW _CONCATENATE(PORT, WRITE_LOW)
#define DDR_WRITE_LOW _CONCATENATE(DDR, WRITE_LOW)
#define PORT_READ_HIGH _CONCATENATE(PORT, READ_HIGH)
#define PIN_READ_HIGH _CONCATENATE(PIN, READ_HIGH)
#define DDR_READ_HIGH _CONCATENATE(DDR, READ_HIGH)
#define PORT_READ_LOW _CONCATENATE(PORT, READ_LOW)
#define PIN_READ_LOW _CONCATENATE(PIN, READ_LOW)
#define DDR_READ_LOW _CONCATENATE(DDR, READ_LOW)
#define _CONCATENATE(reg, letter) _XCONCATENATE(reg, letter)
#define _XCONCATENATE(reg, letter) (reg ## letter)

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
#define PIN_HREAD_READ HIGH
#define PIN_HREAD_WRITE LOW

// Milliseconds to wait for HACK
#define TIMEOUT 100

// We need at least 55; 10 registers * (4 num bytes + 1 space byte) + 1 address byte + 1 space + 1 carriage return + 1 newline + 1 null
char buf[82];
char* bufPtr = buf;

uint8_t hctl = LOW;

void setup() {
  memset(buf, 0, sizeof(buf));
  Serial.begin(9600);
  // Set read ports to inputs
  DDR_READ_HIGH = DDR_READ_LOW = 0x00;
  // Set write ports to outputs
  DDR_WRITE_HIGH = DDR_WRITE_LOW = 0xFF;
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
  hctl ^= 1;
  digitalWrite(PIN_HCTL, hctl);
}

void doWrite() {
  if (*bufPtr != 'w') {
    Serial.print("\n! Write request must begin with w, got: ");
    Serial.print(bufPtr);
    Serial.print("\n");
    return;
  }
  bufPtr++;
  Serial.print("\n! Write request received: ");
  Serial.print(bufPtr);
  Serial.print("\n");
  char* endPtr = NULL;
  long address = strtol(bufPtr, &endPtr, 16);
  // Toggle HCTL to reset address
  digitalWrite(PIN_HREAD, PIN_HREAD_WRITE);
  toggleHctl();
  writeWord(address);
  while (true) {
    bufPtr = endPtr;
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
  PORT_WRITE_LOW = word & 0xFF;
  PORT_WRITE_HIGH = (word >> 8) & 0xFF;
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
    if (*(bufPtr - 1) == '\n') {
      bufPtr = buf;
      doWrite();
      bufPtr = buf;
      memset(buf, 0, sizeof(buf));
    }
  }
  delay(10);
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
    long word = PIN_READ_LOW;
    word |= (PIN_READ_HIGH << 8)
    Serial.print(word, HEX);
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
