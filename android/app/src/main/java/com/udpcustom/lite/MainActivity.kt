package com.udpcustom.lite

import android.os.Bundle
import android.widget.Button
import android.widget.EditText
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity

class MainActivity : AppCompatActivity() {
    private var client: UdpTunnelClient? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        val configInput = findViewById<EditText>(R.id.configInput)
        val dstInput = findViewById<EditText>(R.id.dstInput)
        val serverPortInput = findViewById<EditText>(R.id.serverPortInput)
        val logView = findViewById<TextView>(R.id.logView)
        val startBtn = findViewById<Button>(R.id.startBtn)
        val stopBtn = findViewById<Button>(R.id.stopBtn)

        configInput.setText("min.xhmt.my.id:54-65535@Trial25171:5300")
        dstInput.setText("8.8.8.8:53")
        serverPortInput.setText("9000")

        fun log(msg: String) {
            runOnUiThread {
                logView.append("$msg\n")
            }
        }

        startBtn.setOnClickListener {
            try {
                val cfg = ConfigParser.parse(configInput.text.toString())
                val dst = dstInput.text.toString()
                val idx = dst.lastIndexOf(':')
                require(idx > 0 && idx < dst.length - 1) { "dst format invalid" }
                val dstHost = dst.substring(0, idx)
                val dstPort = dst.substring(idx + 1).toInt()
                val tcpPort = serverPortInput.text.toString().toInt()

                client?.stop()
                client = UdpTunnelClient(cfg, dstHost, dstPort, tcpPort, ::log)
                client?.start()
                log("Tunnel started")
            } catch (e: Exception) {
                log("Error: ${e.message}")
            }
        }

        stopBtn.setOnClickListener {
            client?.stop()
            client = null
        }
    }

    override fun onDestroy() {
        client?.stop()
        super.onDestroy()
    }
}
