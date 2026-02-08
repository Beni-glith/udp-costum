package com.udpcustom.lite

import android.os.Bundle
import android.widget.Button
import android.widget.CheckBox
import android.widget.EditText
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity

class MainActivity : AppCompatActivity() {
    private var client: UdpTunnelClient? = null
    private var connected = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        val configInput = findViewById<EditText>(R.id.configInput)
        val dstInput = findViewById<EditText>(R.id.dstInput)
        val serverPortInput = findViewById<EditText>(R.id.serverPortInput)
        val udpCustomCheck = findViewById<CheckBox>(R.id.udpCustomCheck)
        val sslCheck = findViewById<CheckBox>(R.id.sslCheck)
        val dnsCheck = findViewById<CheckBox>(R.id.dnsCheck)
        val slowDnsCheck = findViewById<CheckBox>(R.id.slowDnsCheck)
        val connectBtn = findViewById<Button>(R.id.connectBtn)
        val statusView = findViewById<TextView>(R.id.statusView)
        val logView = findViewById<TextView>(R.id.logView)

        configInput.setText("turu.kacer.store:1-65535@kacer:vpn")
        dstInput.setText("8.8.8.8:53")
        serverPortInput.setText("9000")
        udpCustomCheck.isChecked = true

        fun log(msg: String) {
            runOnUiThread {
                logView.append("$msg\n")
            }
        }

        fun setConnectedState(v: Boolean) {
            connected = v
            connectBtn.text = if (v) getString(R.string.disconnect) else getString(R.string.connect)
            statusView.text = if (v) getString(R.string.status_connected) else getString(R.string.status_idle)
        }

        setConnectedState(false)

        connectBtn.setOnClickListener {
            if (connected) {
                client?.stop()
                client = null
                setConnectedState(false)
                log("Disconnected")
                return@setOnClickListener
            }

            try {
                if (!udpCustomCheck.isChecked) {
                    log("Aktifkan UDP Custom dulu")
                    return@setOnClickListener
                }

                if (sslCheck.isChecked || dnsCheck.isChecked || slowDnsCheck.isChecked) {
                    log("Info: SSL/DNS/SlowDns belum diimplementasikan (UDP tunnel only)")
                }

                val cfg = ConfigParser.parse(configInput.text.toString())
                val dst = dstInput.text.toString()
                val idx = dst.lastIndexOf(':')
                require(idx > 0 && idx < dst.length - 1) { "dst format invalid" }
                val dstHost = dst.substring(0, idx)
                val dstPort = dst.substring(idx + 1).toInt()
                val tcpPort = serverPortInput.text.toString().ifBlank { "9000" }.toInt()

                client?.stop()
                client = UdpTunnelClient(cfg, dstHost, dstPort, tcpPort, ::log)
                client?.start()
                setConnectedState(true)
                log("Tunnel started")
            } catch (e: Exception) {
                setConnectedState(false)
                val msg = e.message ?: "unknown error"
                log("Error: $msg")
                if (msg.contains("EACCES", ignoreCase = true) || msg.contains("Permission denied", ignoreCase = true)) {
                    log("Hint: gunakan localPort >= 1024, contoh :5300")
                }
            }
        }
    }

    override fun onDestroy() {
        client?.stop()
        super.onDestroy()
    }
}
