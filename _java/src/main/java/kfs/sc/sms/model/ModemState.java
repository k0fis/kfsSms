package kfs.sc.sms.model;

public enum ModemState {
    DISCONNECTED,
    INITIALIZING,
    WAITING_FOR_SIM,
    WAITING_FOR_NETWORK,
    READY,
    ERROR
}
